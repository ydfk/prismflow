#!/usr/bin/env bash
set -euo pipefail
# 该测试会操作容器内的 /usr/local 和 systemd 文件，绝不允许在宿主直接执行。
[[ -f /.dockerenv && ${EUID} -eq 0 ]] || { echo '仅在一次性 root Docker 容器内运行。' >&2; exit 1; }
SOURCE=${1:-/workspace}
command -v jq >/dev/null || { apt-get update -qq; apt-get install -y --no-install-recommends jq; }
WORK=$(mktemp -d);trap 'rm -rf -- "$WORK"' EXIT
export FIXTURES=$WORK/releases MOCK_LOG=$WORK/commands.log MOCK_API=$WORK/api.log
mkdir -p "$FIXTURES" "$WORK/bin"
export PATH="$WORK/bin:$PATH"
cat > "$WORK/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
url= output=
while [[ $# -gt 0 ]];do case "$1" in -o) output=$2;shift 2 ;; https://*) url=$1;shift ;; --retry|--connect-timeout|--max-time) shift 2 ;; *) shift ;; esac;done
printf '%s\n' "$url" >> "$MOCK_API"
if [[ "$url" == */releases/latest || "$url" == */releases/tags/* ]];then
 tag=${url##*/};if [[ "$tag" == latest ]];then tag=$(cat "$FIXTURES/latest");fi
 prerelease=false;if [[ "$tag" == *-* ]];then prerelease=true;fi
 printf '{"tag_name":"%s","draft":false,"prerelease":%s}' "$tag" "$prerelease";exit
fi
tag=$(awk -F/ '{print $(NF-1)}' <<<"$url");asset=${url##*/}
cp "$FIXTURES/$tag/$asset" "$output"
MOCK
cat > "$WORK/bin/systemctl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_LOG"
if [[ "$1" == is-active ]];then
  [[ -f /usr/local/bin/prismflow ]] || exit 1
  installed=$(/usr/local/bin/prismflow version | awk '{print $NF}')
  [[ "$installed" != "${FAIL_VERSION:-}" ]]
fi
MOCK
cat > "$WORK/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s image=%s\n' "$*" "${PRISMFLOW_IMAGE:-}" >> "$MOCK_LOG"
if [[ "$1" == compose && " $* " == *' config '* ]]; then
  printf '{"services":{"prismflow":{"image":"%s"}}}\n' "${MOCK_DOCKER_RESOLVED_IMAGE:-${PRISMFLOW_IMAGE:-}}"
fi
if [[ "$1" == pull && "$2" == "${MOCK_DOCKER_PULL_FAIL:-}" ]]; then exit 1; fi
if [[ "$1" == compose && " $* " == *' up '* && "${PRISMFLOW_IMAGE:-}" == "${MOCK_DOCKER_UP_FAIL_IMAGE:-}" ]]; then exit 1; fi
MOCK
chmod +x "$WORK/bin/"*
fixture(){
 local tag=$1 dir="$WORK/package/prismflow-linux-amd64"
 mkdir -p "$dir/scripts" "$dir/deploy" "$FIXTURES/$tag"
 cp "$SOURCE"/scripts/{install,uninstall,update,release-common}.sh "$dir/scripts/"
 cp "$SOURCE"/deploy/prismflow* "$dir/deploy/"
 printf '#!/usr/bin/env bash\necho "PrismFlow %s"\n' "${tag#v}" > "$dir/prismflow";chmod +x "$dir/prismflow"
 tar -czf "$FIXTURES/$tag/prismflow-linux-amd64.tar.gz" -C "$WORK/package" prismflow-linux-amd64

 cp "$SOURCE/deploy/compose.release.yaml" "$FIXTURES/$tag/compose.release.yaml"
 (cd "$FIXTURES/$tag";sha256sum *.tar.gz compose.release.yaml > SHA256SUMS)
}
for tag in v1.0.0 v1.1.0 v1.2.0 v1.3.0 v1.3.0-rc.1;do fixture "$tag";done
printf 'v1.0.0\n' > "$FIXTURES/latest"
bash "$SOURCE/scripts/bootstrap.sh" --mode native
[[ $(/usr/local/bin/prismflow version) == 'PrismFlow 1.0.0' ]]
[[ -f /etc/systemd/system/prismflow-update.path && -f /etc/systemd/system/prismflow-update.timer ]]
printf 'preserve-data' > /var/lib/prismflow/sentinel
# 默认关闭自动更新时，timer 不应发起网络请求。
before=$(wc -l < "$MOCK_API");/usr/local/bin/prismflow-update run;[[ $(wc -l < "$MOCK_API") == "$before" ]]
/usr/local/bin/prismflow-update auto on
[[ -f /var/lib/prismflow-updater/requests/auto-enabled ]]
runuser -u prismflow -- test ! -w /var/lib/prismflow-updater
runuser -u prismflow -- sh -c 'printf stable > /var/lib/prismflow-updater/requests/request'
printf 'v1.1.0\n' > "$FIXTURES/latest"
/usr/local/bin/prismflow-update run
[[ $(/usr/local/bin/prismflow version) == 'PrismFlow 1.1.0' ]]
[[ $(cat /var/lib/prismflow/sentinel) == preserve-data ]]
jq -e '.state=="done"' /var/lib/prismflow-updater/status.json >/dev/null
# 新程序启动失败必须恢复原程序，而不是仅报告更新成功。
printf 'v1.2.0\n' > "$FIXTURES/latest"
if FAIL_VERSION=1.2.0 /usr/local/bin/prismflow-update now;then echo '未拒绝启动失败版本' >&2;exit 1;fi
[[ $(/usr/local/bin/prismflow version) == 'PrismFlow 1.1.0' ]]
jq -e '.state=="failed"' /var/lib/prismflow-updater/status.json >/dev/null
# 校验失败不替换程序，也不停止原服务。
printf 'v1.3.0\n' > "$FIXTURES/latest";printf 'tamper' >> "$FIXTURES/v1.3.0/prismflow-linux-amd64.tar.gz"
before=$(grep -c '^stop prismflow$' "$MOCK_LOG")
if /usr/local/bin/prismflow-update now;then echo '未拒绝损坏安装包' >&2;exit 1;fi
[[ $(/usr/local/bin/prismflow version) == 'PrismFlow 1.1.0' && $(grep -c '^stop prismflow$' "$MOCK_LOG") == "$before" ]]
/usr/local/bin/prismflow-update auto off
[[ ! -f /var/lib/prismflow-updater/requests/auto-enabled ]]
# 跨模式不能覆盖原生服务，卸载保留数据，再模拟 Docker 安装。
if bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo '允许覆盖原生部署' >&2;exit 1;fi
bash "$SOURCE/scripts/uninstall.sh"
printf 'v1.0.0\n' > "$FIXTURES/latest"
bash "$SOURCE/scripts/bootstrap.sh" --mode docker
[[ -f /opt/prismflow/compose.yaml && $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.0.0 ]]
printf 'docker-data' > /opt/prismflow/data/sentinel
# 模拟旧的 docker load 安装，升级必须保留 Compose 与数据并切换为 Hub 镜像。
printf 'prismflow:1.0.0\n' > /opt/prismflow/image-version
printf '\n# user-customization\n' >> /opt/prismflow/compose.yaml
compose_hash=$(sha256sum /opt/prismflow/compose.yaml)
printf 'v1.1.0\n' > "$FIXTURES/latest"
bash "$SOURCE/scripts/bootstrap.sh" --mode docker
[[ $(cat /opt/prismflow/data/sentinel) == docker-data ]]
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.1.0 && $(sha256sum /opt/prismflow/compose.yaml) == "$compose_hash" ]]
grep -q 'docker pull ydfk/prismflow:1.1.0' "$MOCK_LOG"
if grep -q '^docker load ' "$MOCK_LOG"; then echo '仍然使用离线镜像导入' >&2;exit 1;fi
# Hub 不可用或现有 Compose 指向其他镜像时，不能替换部署记录或停止原容器。
printf 'v1.2.0\n' > "$FIXTURES/latest"
before=$(grep -c ' up -d ' "$MOCK_LOG")
if MOCK_DOCKER_PULL_FAIL=ydfk/prismflow:1.2.0 bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo '未拒绝拉取失败' >&2;exit 1;fi
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.1.0 && $(grep -c ' up -d ' "$MOCK_LOG") == "$before" ]]
if MOCK_DOCKER_RESOLVED_IMAGE=custom/image:stable bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo '未拒绝硬编码的其他镜像' >&2;exit 1;fi
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.1.0 ]]
if MOCK_DOCKER_UP_FAIL_IMAGE=ydfk/prismflow:1.2.0 bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo '未拒绝启动命令失败' >&2;exit 1;fi
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.1.0 && $(cat /opt/prismflow/data/sentinel) == docker-data ]]
grep -q 'up -d --no-build --pull never image=ydfk/prismflow:1.1.0' "$MOCK_LOG"
# Compose 仍从 Release 校验，镜像改成 Hub 不应放宽安装文件校验。
printf '\n# tamper\n' >> "$FIXTURES/v1.2.0/compose.release.yaml"
if bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo '未拒绝损坏的 Compose 资产' >&2;exit 1;fi
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.1.0 ]]
# latest 不接受预发布；显式指定则拉取对应 Hub 标签。
printf 'v1.3.0-rc.1\n' > "$FIXTURES/latest"
if bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo 'latest 接受了预发布' >&2;exit 1;fi
bash "$SOURCE/scripts/bootstrap.sh" --mode docker --version v1.3.0-rc.1
[[ $(cat /opt/prismflow/image-version) == ydfk/prismflow:1.3.0-rc.1 && $(cat /opt/prismflow/data/sentinel) == docker-data ]]
# Docker 不再发布 ARM64，安装器必须在下载或系统修改之前拒绝。
cat > "$WORK/bin/uname" <<'MOCK'
#!/usr/bin/env bash
if [[ "$1" == -m ]];then echo aarch64;else /usr/bin/uname "$@";fi
MOCK
chmod +x "$WORK/bin/uname"
before=$(wc -l < "$MOCK_API")
if bash "$SOURCE/scripts/bootstrap.sh" --mode docker;then echo 'ARM64 Docker 安装未被拒绝' >&2;exit 1;fi
[[ $(wc -l < "$MOCK_API") == "$before" ]]
echo 'PASS: native updates; amd64 Docker Hub install/migration/recovery; ARM64 Docker rejection; checksums and data preservation'
