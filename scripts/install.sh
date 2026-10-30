#!/usr/bin/env bash
set -euo pipefail
if [[ ${EUID} -ne 0 ]]; then echo '请使用 sudo 运行安装脚本。' >&2; exit 1; fi
if [[ ! -f /etc/os-release ]]; then echo '不支持此系统。' >&2; exit 1; fi
source /etc/os-release
case "${ID}:${VERSION_ID}" in debian:12|debian:13|ubuntu:22.04|ubuntu:24.04) ;; *) echo '直接安装仅支持 Debian 12/13 和 Ubuntu 22.04/24.04。' >&2; exit 1 ;; esac
command -v systemctl >/dev/null || { echo '需要 systemd。' >&2; exit 1; }
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BINARY=${1:-"$ROOT/prismflow"}
[[ -f "$BINARY" ]] || { echo "未找到二进制文件：$BINARY" >&2; exit 1; }
"$BINARY" version
if ! command -v curl >/dev/null || ! command -v openssl >/dev/null || ! command -v jq >/dev/null; then
  apt-get update
  apt-get install -y --no-install-recommends ca-certificates curl openssl jq util-linux
fi
id prismflow >/dev/null 2>&1 || useradd --system --home /var/lib/prismflow --shell /usr/sbin/nologin prismflow
install -d -m 0755 /opt/prismflow /etc/prismflow
install -d -o prismflow -g prismflow -m 0700 /var/lib/prismflow
if [[ -f /usr/local/bin/prismflow ]]; then
  cp -p /usr/local/bin/prismflow /usr/local/bin/prismflow.previous
  systemctl stop prismflow || true
fi
install -m 0755 "$BINARY" /usr/local/bin/prismflow
# 内核可在首次登录后由面板自动下载到服务数据目录，无需安装阶段联网。
install -d -m 0755 /usr/local/lib/prismflow /var/lib/prismflow-updater
install -d -o prismflow -g prismflow -m 0750 /var/lib/prismflow-updater/requests
install -m 0755 "$ROOT/scripts/release-common.sh" "$ROOT/scripts/update.sh" /usr/local/lib/prismflow/
cat > /usr/local/bin/prismflow-update <<'UPDATER'
#!/usr/bin/env bash
exec /usr/local/lib/prismflow/update.sh "$@"
UPDATER
chmod 0755 /usr/local/bin/prismflow-update
install -m 0644 "$ROOT/deploy/prismflow.service" "$ROOT/deploy/prismflow-update.service" "$ROOT/deploy/prismflow-update.path" "$ROOT/deploy/prismflow-update.timer" /etc/systemd/system/
if [[ ! -f /etc/prismflow/environment ]]; then
  cat > /etc/prismflow/environment <<'ENV'
PRISMFLOW_LISTEN=127.0.0.1:8080
PRISMFLOW_PUBLIC_LISTEN=127.0.0.1:8081
# 使用 HTTPS 反向代理时填写实际管理域名：
# PRISMFLOW_ORIGIN=https://panel.example.com
ENV
  chmod 0600 /etc/prismflow/environment
fi
systemctl daemon-reload
systemctl enable --now prismflow
systemctl enable --now prismflow-update.path prismflow-update.timer
echo '安装完成。通过 SSH 转发或 HTTPS 反向代理访问管理端口 8080。'
echo '初始化令牌：sudo cat /var/lib/prismflow/setup.token'
echo '状态：sudo systemctl status prismflow'
