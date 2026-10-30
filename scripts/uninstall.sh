#!/usr/bin/env bash
set -euo pipefail
[[ ${EUID} -eq 0 ]] || { echo '请使用 sudo 运行。' >&2; exit 1; }
systemctl disable --now prismflow-update.path prismflow-update.timer 2>/dev/null || true
systemctl stop prismflow-update.service 2>/dev/null || true
systemctl disable --now prismflow
rm -f /etc/systemd/system/prismflow.service /etc/systemd/system/prismflow-update.service /etc/systemd/system/prismflow-update.path /etc/systemd/system/prismflow-update.timer /usr/local/bin/prismflow /usr/local/bin/prismflow-update
rm -f /usr/local/lib/prismflow/release-common.sh /usr/local/lib/prismflow/update.sh
systemctl daemon-reload
echo '已移除服务与主程序。数据、配置、内核及 previous 备份均保留：'
echo '/var/lib/prismflow /etc/prismflow /opt/prismflow /var/lib/prismflow-updater /usr/local/bin/prismflow.previous'
