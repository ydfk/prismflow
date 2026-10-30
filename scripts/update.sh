#!/usr/bin/env bash
set -Eeuo pipefail
HERE=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$HERE/release-common.sh"
BINARY=/usr/local/bin/prismflow
CONTROL=/var/lib/prismflow-updater
REQUESTS=$CONTROL/requests
WORK= SWITCHED=0 COMMITTED=0 TARGET= CURRENT=
current_version() { "$BINARY" version | awk '{print $NF}'; }
status() {
  local state=$1 message=$2 temporary
  temporary=$(mktemp "$CONTROL/.status.XXXXXX")
  jq -n --arg state "$state" --arg message "$message" --arg current "${CURRENT:-}" --arg latest "${TARGET:-}" --arg checkedAt "$(date -u +%FT%TZ)" '{state:$state,message:$message,current:$current,latest:$latest,checkedAt:$checkedAt}' > "$temporary"
  chmod 0644 "$temporary"; mv -Tf -- "$temporary" "$CONTROL/status.json"
}
replace_binary() {
  local source=$1 temporary
  temporary=$(mktemp /usr/local/bin/.prismflow.XXXXXX)
  install -m 0755 "$source" "$temporary" || { rm -f -- "$temporary"; return 1; }
  mv -Tf -- "$temporary" "$BINARY"
}
failure() {
  trap - ERR
  local message='更新失败，保留现有版本。'
  if [[ "$SWITCHED" == 1 && "$COMMITTED" == 0 ]]; then
    if replace_binary "$BINARY.previous" && systemctl restart prismflow; then
      message='新版本启动失败，已恢复原程序。'
    else message='新版本启动失败，原程序恢复异常，请检查 systemd 日志。'; fi
  fi
  CURRENT=$(current_version 2>/dev/null || true)
  status failed "$message" || true
  echo "$message" >&2
  exit 1
}
cleanup() { if [[ -n "$WORK" ]]; then rm -rf -- "$WORK"; fi; }
update_now() {
  CURRENT=$(current_version)
  local comparable=${CURRENT/-/\~}
  TARGET=$(release_tag latest)
  if [[ "$CURRENT" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] && ! dpkg --compare-versions "${TARGET#v}" gt "$comparable"; then
    status idle '当前已是最新稳定版，未修改程序。'; return
  fi
  status downloading '正在下载并校验最新稳定版。'
  local arch asset extracted expected actual
  arch=$(release_arch); asset="prismflow-linux-$arch.tar.gz"
  WORK=$(mktemp -d)
  release_download "$TARGET" SHA256SUMS "$WORK/SHA256SUMS"
  release_download "$TARGET" "$asset" "$WORK/$asset"
  release_verify "$WORK" "$asset"
  release_extract "$WORK" "$asset" "$arch"
  extracted="$WORK/prismflow-linux-$arch"
  expected=${TARGET#v}; actual=$("$extracted/prismflow" version | awk '{print $NF}')
  [[ "$actual" == "$expected" ]] || { echo '程序版本与 Release 不一致。' >&2; return 1; }
  cp --remove-destination --preserve=mode -- "$BINARY" "$BINARY.previous"
  status applying '校验通过，正在替换程序并重启服务。'
  systemctl stop prismflow
  SWITCHED=1
  replace_binary "$extracted/prismflow"
  systemctl start prismflow
  # 连续检查稳定运行，避免把瞬时 active 当作启动成功。
  for _ in 1 2 3; do sleep 1; systemctl is-active --quiet prismflow; done
  # 主程序正常后再原子更新 root 辅助脚本，当前 shell 仍读取旧 inode。
  for file in release-common.sh update.sh; do
    install -m 0755 "$extracted/scripts/$file" "/usr/local/lib/prismflow/$file.new"
    mv -Tf -- "/usr/local/lib/prismflow/$file.new" "/usr/local/lib/prismflow/$file"
  done
  COMMITTED=1; CURRENT=$expected
  status done '面板已更新，数据与内核保持原样。'
}
main() {
  local action=${1:-check}
  if [[ "$action" == check ]]; then
    printf '当前版本：%s\n最新稳定版：%s\n' "$(current_version)" "$(release_tag latest)"; return
  fi
  [[ ${EUID} -eq 0 ]] || { echo '更新操作需要 sudo。' >&2; return 1; }
  if [[ "$action" == auto ]]; then
    case "${2:-}" in
      on) local temporary; temporary=$(mktemp "$CONTROL/.auto.XXXXXX"); chown prismflow:prismflow "$temporary"; chmod 0600 "$temporary"; mv -Tf -- "$temporary" "$REQUESTS/auto-enabled"; echo '已启用：每小时检查稳定版。' ;;
      off) rm -f -- "$REQUESTS/auto-enabled"; echo '已关闭自动更新。' ;;
      *) echo '用法：prismflow-update auto on|off' >&2; return 1 ;;
    esac
    return
  fi
  [[ "$action" == now || "$action" == run ]] || { echo '用法：prismflow-update check|now|auto on|auto off' >&2; return 1; }
  exec 9>/run/lock/prismflow-update.lock
  flock -w 300 9
  if [[ "$action" == run && ! -f "$REQUESTS/request" && ! -f "$REQUESTS/auto-enabled" ]]; then return; fi
  rm -f -- "$REQUESTS/request"
  trap cleanup EXIT
  trap failure ERR
  update_now
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
