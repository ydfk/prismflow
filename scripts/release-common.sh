#!/usr/bin/env bash
# 由安装器和 root 更新服务共享；不读取或执行面板提供的命令。
set -euo pipefail
REPOSITORY=ydfk/prismflow
release_tag() {
  local requested=${1:-latest} endpoint result
  endpoint="https://api.github.com/repos/$REPOSITORY/releases/latest"
  if [[ "$requested" != latest ]]; then
    [[ "$requested" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo '版本格式应为 v1.2.3。' >&2; return 1; }
    endpoint="https://api.github.com/repos/$REPOSITORY/releases/tags/$requested"
  fi
  result=$(curl --fail --silent --show-error --location --retry 2 --connect-timeout 15 --max-time 60 "$endpoint") || { echo '未找到可用 Release；请先发布版本 Tag。' >&2; return 1; }
  jq -e '.draft == false and (.tag_name | type == "string")' <<<"$result" >/dev/null
  if [[ "$requested" == latest ]]; then jq -e '.prerelease == false' <<<"$result" >/dev/null; fi
  local tag; tag=$(jq -r .tag_name <<<"$result")
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || return 1
  printf '%s\n' "$tag"
}
release_arch() {
  case "$(uname -m)" in x86_64|amd64) echo amd64 ;; aarch64|arm64) echo arm64 ;; *) echo '仅支持 Linux amd64 / arm64。' >&2; return 1 ;; esac
}
release_download() {
  local tag=$1 asset=$2 destination=$3
  curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 --max-time 600 "https://github.com/$REPOSITORY/releases/download/$tag/$asset" -o "$destination"
}
release_verify() {
  local directory=$1 asset=$2 expected
  expected=$(awk -v file="$asset" '$2 == file && $1 ~ /^[0-9a-f]+$/ && length($1)==64 {print $1}' "$directory/SHA256SUMS")
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || { echo '校验清单缺少目标文件或存在重复条目。' >&2; return 1; }
  printf '%s  %s\n' "$expected" "$directory/$asset" | sha256sum -c -
}
release_extract() {
  local directory=$1 archive=$2 arch=$3 entry
  while IFS= read -r entry; do
    [[ "$entry" == "prismflow-linux-$arch/"* && "$entry" != *'/../'* && "$entry" != *'/./'* ]] || { echo '安装包路径不安全。' >&2; return 1; }
  done < <(tar -tzf "$directory/$archive")
  tar --no-same-owner --no-same-permissions -xzf "$directory/$archive" -C "$directory"
}
