#!/usr/bin/env bash
set -euo pipefail
MODE= REQUESTED_VERSION=latest AUTO=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE=${2:?缺少安装方式}; shift 2 ;;
    --version) REQUESTED_VERSION=${2:?缺少版本}; shift 2 ;;
    --auto-update) AUTO=1; shift ;;
    -h|--help) echo '用法：bash bootstrap.sh [--mode native|docker] [--version v1.2.3] [--auto-update]'; exit 0 ;;
    *) echo "未知参数：$1" >&2; exit 1 ;;
  esac
done
[[ ${EUID} -eq 0 ]] || { echo '请通过 sudo bash 执行安装器。' >&2; exit 1; }
[[ $(uname -s) == Linux ]] || { echo '安装器仅支持 Linux。' >&2; exit 1; }
if [[ -z "$MODE" ]]; then
  if { : </dev/tty; } 2>/dev/null; then
    printf '\nPrismFlow · 棱流\n  1) 原生版本（systemd）\n  2) Docker 版本\n选择 [1/2，默认 1]：' >/dev/tty
    read -r selection </dev/tty
    case "${selection:-1}" in 1) MODE=native ;; 2) MODE=docker ;; *) echo '无效选项。' >&2; exit 1 ;; esac
  else echo '非交互环境请指定 --mode native 或 --mode docker。' >&2; exit 1; fi
fi
[[ "$MODE" == native || "$MODE" == docker ]] || { echo '安装方式应为 native 或 docker。' >&2; exit 1; }
if [[ "$MODE" == docker && "$AUTO" == 1 ]]; then echo '--auto-update 仅适用于原生版本。' >&2; exit 1; fi
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo '仅支持 amd64 / arm64。' >&2; exit 1 ;; esac
if [[ "$MODE" == docker && "$ARCH" != amd64 ]]; then echo 'Docker 镜像暂时仅支持 x86_64（amd64）；ARM64 请使用原生安装。' >&2; exit 1; fi
source /etc/os-release
supported() { case "$ID:$VERSION_ID" in debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) return 0 ;; *) return 1 ;; esac; }
if [[ "$MODE" == native ]]; then
  supported || { echo '原生安装支持 Debian 12/13、Ubuntu 22.04/24.04。' >&2; exit 1; }
  if [[ -f /opt/prismflow/compose.yaml ]]; then echo '检测到 Docker 部署；请先完成迁移，不能覆盖安装。' >&2; exit 1; fi
else
  if [[ -f /etc/systemd/system/prismflow.service ]] || systemctl is-active --quiet prismflow 2>/dev/null; then echo '检测到原生服务；请先备份并卸载原生服务后再迁移。' >&2; exit 1; fi
fi
if ! command -v jq >/dev/null || ! command -v curl >/dev/null; then
  supported || { echo '请先安装 curl、jq，再重新运行。' >&2; exit 1; }
  apt-get update; apt-get install -y --no-install-recommends ca-certificates curl jq
fi
WORK=$(mktemp -d); trap 'rm -rf -- "$WORK"' EXIT
# 安装器独立解析 Release；后续下载的每个安装资源都校验发布清单。
endpoint=https://api.github.com/repos/ydfk/prismflow/releases/latest
if [[ "$REQUESTED_VERSION" != latest ]]; then
  [[ "$REQUESTED_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo '版本格式应为 v1.2.3。' >&2; exit 1; }
  endpoint="https://api.github.com/repos/ydfk/prismflow/releases/tags/$REQUESTED_VERSION"
fi
metadata=$(curl -fsSL --retry 2 --connect-timeout 15 --max-time 60 "$endpoint") || { echo '尚无可下载的 Release，请先发布版本 Tag。' >&2; exit 1; }
jq -e '.draft == false and (.tag_name | type == "string")' <<<"$metadata" >/dev/null
if [[ "$REQUESTED_VERSION" == latest ]]; then jq -e '.prerelease == false' <<<"$metadata" >/dev/null; fi
TAG=$(jq -r .tag_name <<<"$metadata")
[[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || exit 1
BASE="https://github.com/ydfk/prismflow/releases/download/$TAG"
fetch() { curl -fsSL --retry 3 --connect-timeout 15 --max-time 600 "$BASE/$1" -o "$WORK/$1"; }
verify() {
  local expected; expected=$(awk -v file="$1" '$2==file && length($1)==64 && $1 ~ /^[0-9a-f]+$/ {print $1}' "$WORK/SHA256SUMS")
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || { echo "缺少可信校验项：$1" >&2; exit 1; }
  printf '%s  %s\n' "$expected" "$WORK/$1" | sha256sum -c -
}
fetch SHA256SUMS
if [[ "$MODE" == native ]]; then
  ASSET="prismflow-linux-$ARCH.tar.gz"; fetch "$ASSET"; verify "$ASSET"
  while IFS= read -r entry; do [[ "$entry" == "prismflow-linux-$ARCH/"* && "$entry" != *'/../'* ]] || { echo '不安全的安装包路径。' >&2; exit 1; }; done < <(tar -tzf "$WORK/$ASSET")
  tar --no-same-owner --no-same-permissions -xzf "$WORK/$ASSET" -C "$WORK"
  bash "$WORK/prismflow-linux-$ARCH/scripts/install.sh" "$WORK/prismflow-linux-$ARCH/prismflow"
  if [[ "$AUTO" == 1 ]]; then /usr/local/bin/prismflow-update auto on; fi
  cat /var/lib/prismflow/setup.token 2>/dev/null || true
else
  if ! command -v docker >/dev/null; then
    supported || { echo '此系统需先安装 Docker Engine 与 Compose 插件。' >&2; exit 1; }
    apt-get update; apt-get install -y --no-install-recommends ca-certificates curl
    install -d -m 0755 /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/$ID/gpg" -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    codename=${UBUNTU_CODENAME:-$VERSION_CODENAME}
    printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/%s %s stable\n' "$(dpkg --print-architecture)" "$ID" "$codename" > /etc/apt/sources.list.d/docker.list
    apt-get update; apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    systemctl enable --now docker
  fi
  docker compose version >/dev/null || { echo '请安装 Docker Compose 插件后重试。' >&2; exit 1; }
  fetch compose.release.yaml; verify compose.release.yaml
  IMAGE="ydfk/prismflow:${TAG#v}"
  COMPOSE="$WORK/compose.release.yaml"
  if [[ -f /opt/prismflow/compose.yaml ]]; then COMPOSE=/opt/prismflow/compose.yaml; fi
  resolved=$(PRISMFLOW_IMAGE="$IMAGE" docker compose -f "$COMPOSE" config --format json)
  jq -e --arg image "$IMAGE" '.services.prismflow.image == $image' <<<"$resolved" >/dev/null || { echo '现有 Compose 未使用 PRISMFLOW_IMAGE；请先调整镜像配置，原部署未改动。' >&2; exit 1; }
  # 成功拉取后才操作部署；当前 Docker 镜像仅提供 x86_64 / amd64。
  docker pull "$IMAGE"
  install -d -m 0755 /opt/prismflow
  if [[ ! -d /opt/prismflow/data ]]; then install -d -o 10001 -g 10001 -m 0700 /opt/prismflow/data; fi
  if [[ ! -f /opt/prismflow/compose.yaml ]]; then install -m 0644 "$WORK/compose.release.yaml" /opt/prismflow/compose.yaml; fi
  previous_image=$(cat /opt/prismflow/image-version 2>/dev/null || true)
  if ! PRISMFLOW_IMAGE="$IMAGE" docker compose -f /opt/prismflow/compose.yaml up -d --no-build --pull never; then
    if [[ -n "$previous_image" ]]; then
      PRISMFLOW_IMAGE="$previous_image" docker compose -f /opt/prismflow/compose.yaml up -d --no-build --pull never || echo '旧镜像恢复失败，请检查 docker compose logs。' >&2
    fi
    echo 'Docker 启动命令失败，镜像版本记录未更新。' >&2; exit 1
  fi
  # 保留用户环境、Compose 和数据；成功提交后原子记录完整 Hub 镜像名。
  image_file=$(mktemp /opt/prismflow/.image-version.XXXXXX)
  printf '%s\n' "$IMAGE" > "$image_file"; chmod 0644 "$image_file"
  mv -f "$image_file" /opt/prismflow/image-version
  echo "Docker Hub 部署完成：$IMAGE；数据目录：/opt/prismflow/data"
  PRISMFLOW_IMAGE="$IMAGE" docker compose -f /opt/prismflow/compose.yaml exec -T prismflow sh -c 'cat /data/setup.token 2>/dev/null || true'
fi
echo '管理端默认 127.0.0.1:8080。请使用 SSH 隧道或 HTTPS 反向代理访问。'
