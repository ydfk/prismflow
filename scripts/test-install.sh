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
if [[ "$url" == */releases/latest || "$url" == */releases/tags/* ]];then tag=$(cat "$FIXTURES/latest");printf '{"tag_name":"%s","draft":false,"prerelease":false}' "$tag";exit;fi
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
printf 'docker %s image=%s\n' "$*" "${PRISMFLOW_IMAGE:-}" >> "$MOCK_LOG"
MOCK
chmod +x "$WORK/bin/"*
fixture(){
 local tag=$1 dir="$WORK/package/prismflow-linux-amd64"
 mkdir -p "$dir/scripts" "$dir/deploy" "$FIXTURES/$tag"
 cp "$SOURCE"/scripts/{install,uninstall,update,release-common}.sh "$dir/scripts/"
 cp "$SOURCE"/deploy/prismflow* "$dir/deploy/"
 printf '#!/usr/bin/env bash\necho "PrismFlow %s"\n' "${tag#v}" > "$dir/prismflow";chmod +x "$dir/prismflow"
 tar -czf "$FIXTURES/$tag/prismflow-linux-amd64.tar.gz" -C "$WORK/package" prismflow-linux-amd64
 printf 'docker fixture\n' | gzip > "$FIXTURES/$tag/prismflow-docker-amd64.tar.gz"
 cp "$SOURCE/deploy/compose.release.yaml" "$FIXTURES/$tag/compose.release.yaml"
 (cd "$FIXTURES/$tag";sha256sum *.tar.gz compose.release.yaml > SHA256SUMS)
}
for tag in v1.0.0 v1.1.0 v1.2.0 v1.3.0;do fixture "$tag";done
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
[[ -f /opt/prismflow/compose.yaml && $(cat /opt/prismflow/image-version) == prismflow:1.0.0 ]]
printf 'docker-data' > /opt/prismflow/data/sentinel
printf 'v1.1.0\n' > "$FIXTURES/latest"
bash "$SOURCE/scripts/bootstrap.sh" --mode docker
[[ $(cat /opt/prismflow/data/sentinel) == docker-data ]]
grep -q 'image=prismflow:1.1.0' "$MOCK_LOG"
echo 'PASS: native/docker install, opt-in updates, rollback, checksum rejection, privilege boundary and data preservation'
