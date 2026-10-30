# 域名、反代与证书

推荐沿用服务器已有的 Caddy / Nginx。DNS 只负责把域名指向 VPS；反代处理网页 HTTPS，Mihomo 负责代理协议。PrismFlow 不接管已有反代文件、80/443 端口或原证书的续期任务。

## 先确定三个地址

在「域名与证书 → 公网访问设置」填写，基础域名只是可选的建议地址生成器：

| 地址 | 示例 | 实际上游 |
| --- | --- | --- |
| 面板公网地址 | `https://panel.example.com` | `127.0.0.1:8080`，管理界面与 API |
| 公开订阅地址 | `https://sub.example.com` | `127.0.0.1:8081`，订阅与可选网站 |
| 默认节点地址 | `node.example.com` | 由具体协议决定端口和 TLS 方式 |

`*.example.com` 的 A/AAAA 记录可以指向同一 VPS，但 DNS 泛解析不会自动创建 HTTPS 站点，也不代表已经有泛域名证书。若 AAAA 指向的服务器不可达，客户端可能连接失败；Cloudflare 的普通 HTTP 代理不能直接承载 REALITY、AnyTLS 或 Hysteria2，应按协议使用 DNS-only 记录。

三个地址可使用不同的独立域名，无需共享基础域名。面板与订阅使用不同的域名或端口；不支持以 URL 子目录部署。无域名时可以仅用 SSH 隧道管理面板、创建 REALITY 等无需自有证书的入口。

保存只影响后续新建默认值，不会批量改写已有节点、SNI 或证书。旧数据的 `defaultHost`、`defaultCertificateId`、`publicUrl` 保留；缺少 TLS 方式的历史入口仍按原方式直连。

## 选择由谁处理 TLS

| 协议 | 客户端连接 | TLS 终止与证书 |
| --- | --- | --- |
| VLESS REALITY Vision | `node.example.com:随机五位端口/TCP` | Mihomo REALITY；无需自有证书；握手目标和 SNI 独立配置 |
| VLESS XHTTP · 外部 TLS | `node.example.com:443`，或指定公网端口 | Caddy / Nginx 维护证书，按路径转发到 `127.0.0.1:随机五位端口` |
| VLESS XHTTP · 内核 TLS | `node.example.com:随机五位端口/TCP` | Mihomo 使用绑定的证书 |
| AnyTLS | `node.example.com:随机五位端口/TCP` | Mihomo 使用绑定的证书 |
| Hysteria2 | `node.example.com:随机五位端口/UDP` | Mihomo 使用绑定的证书 |

新安装默认建议 XHTTP 外部 TLS，可在快速创建或普通创建中切换。外部 TLS 仅开放表单 VLESS + XHTTP + TLS：客户端订阅仍启用 TLS 和证书验证，服务端的明文接收严格限制在回环地址。订阅导出公网端口，入口表格同时显示本机端口；内部端口无需对公网开放。

普通 HTTP 反代不会自动代理任意 TCP/UDP 协议。除外部 XHTTP 外，各协议仍使用自己的端口；多个 XHTTP 可在同一公网域名和端口使用不同路径，路径不能相同或互相包含。通用 L4 分流、跨主机明文上游和所有协议共用 443 不在此方案中。

## 使用已有 Caddy / Nginx

1. 先保存公网访问设置，再创建需要的外部 TLS XHTTP 入口。
2. 打开「反代配置与检查」，复制 Caddy 或 Nginx 片段，合并到现有站点中。同域名已有 server / site 时合并路由，不重复定义；不要整体覆盖已有配置。
3. Caddy 可沿用自动 HTTPS，也可在现有站点沿用已配置的 `tls` 证书。Nginx 片段中的 `/etc/ssl/prismflow/域名/` 是占位路径，必须替换为现有证书文件。
4. XHTTP 路径必须原样转发，不剥离前缀；Nginx 公网监听需启用 HTTP/2，关闭请求与响应缓冲。片段采用兼容旧版的 `listen 443 ssl http2`；新版本可按官方语法调整。
5. 校验配置后重载，再点击「检查公网访问」。入口本身在协议列表单独检测。

```bash
# 使用实际的配置路径；选择已经安装的反代执行
sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
sudo systemctl reload caddy
```

```bash
sudo nginx -t
sudo systemctl reload nginx
```

面板保存的公网地址用于来源校验。若环境中已有 `PRISMFLOW_ORIGIN`，它的优先级更高；修改 `/etc/prismflow/environment` 或 Compose 环境后重启服务。首次可经 SSH 隧道设置，保存面板域名后，本机回环连接的 localhost / 回环 IP 隧道仍可继续配置；显式设置 ORIGIN 时遵循该环境策略。反代就绪后通过新的面板地址登录。设置错误时仍可通过 SSH 修正环境变量恢复访问。

生成的“服务环境”是部署建议，不会自动更改监听地址；已有原生安装和生产 Docker 默认就是回环监听。Docker 使用 Linux host 网络，反代也应位于宿主或同一 host 网络；bridge 容器内的 `127.0.0.1` 不是宿主机。

检查会验证 DNS、TLS 信任与 PrismFlow 的服务角色标识，拒绝跟随重定向，避免把登录页或错误上游判断为正常；网站关闭时标识仍可用。结果只代表服务器视角，公网客户端可达性需实测。XHTTP 节点域名的未匹配路径默认 404；伪装网站展示在订阅域名首页，需要其他网站回落时自行合并对应规则。

## 复用证书，不重复管理

- **仅网页及外部 XHTTP**：由反代维护证书，不需要把私钥交给 PrismFlow。
- **AnyTLS / Hysteria2 等内核 TLS**：选择「使用系统证书」，填写已有证书链与私钥的绝对路径；原工具继续续期，PrismFlow 每 30 秒检测文件变更并安全应用，失败时保留原运行证书。
- **希望由面板签发节点证书**：选择 acme.sh 或内置 ACME。普通域名可使用 HTTP-01 或 Cloudflare DNS-01；`*.example.com` 只能使用 DNS-01。泛域名只覆盖一级子域，不自动覆盖 `example.com` 或多级子域；客户端 SNI 需要具体域名。

证书表格展示实际 SAN 范围和所绑定的托管节点。可以关联覆盖多个节点的同一张泛域名证书；绑定后仍由后端校验证书是否覆盖具体 SNI。

Caddy 私有数据目录不保证其他服务可读，不应简单放开整个目录权限。优先由原续期工具的 deploy hook 将证书原子更新到专用共享目录，并给所需服务账户最小读取权限。PrismFlow 的系统路径关联不会修改原文件或接管续期。

原生服务账户为 `prismflow`，需要读取文件及遍历上层目录；systemd `ProtectHome` 默认限制 home 目录。Docker 使用 UID 10001，需只读挂载证书目录并具备读取权限；必要时通过 `group_add` 加入已有证书读取组的数值 GID。不要把私钥设为全局可读。

```yaml
# 合并到现有 prismflow 服务的 volumes；live 链接还依赖 archive 目录
volumes:
  - ./data:/data
  - /etc/letsencrypt:/etc/letsencrypt:ro
```

面板自己的 HTTP-01 申请还需要将对应域名公网 80 端口的 `/.well-known/acme-challenge/` 转发到 8081。已有反代为同域名续期时，优先直接复用它的证书或改用 DNS-01，避免两套工具争用挑战路径。

依据：[Caddy reverse_proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)、[Nginx proxy 模块](https://nginx.org/en/docs/http/ngx_http_proxy_module.html)、[Let's Encrypt 验证方式](https://letsencrypt.org/docs/challenge-types/)。
