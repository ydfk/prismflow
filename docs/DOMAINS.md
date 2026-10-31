# 统一域名、路径与证书

PrismFlow 的网页只需一个域名和一个公开 HTTPS 端口，不要求端口必须是 443。Caddy/Nginx 负责 HTTPS 与转发，PrismFlow 负责网站、管理和订阅的路径分流。

## 入口约定

以 `https://example.com` 为例：

| 地址 | 用途 |
| --- | --- |
| `https://example.com/` | 伪装网站；关闭网站时此路径返回 404 |
| `https://example.com/manage/` | 管理面板；管理路径可修改 |
| `https://example.com/s/用户令牌` | Mihomo 订阅；追加 `?format=links` 获取分享链接 |

在「域名与证书 → 统一域名与证书」填写统一访问地址和管理路径。新协议默认使用这个域名；如需迁移已有协议，显式勾选“同时更新已有协议的地址与反代端口”。管理路径不是密码，管理员登录、会话与 CSRF 校验仍然生效。修改管理路径后旧路径失效；新地址需要以 `/` 结尾，服务会自动重定向。

浏览器、前端静态资源和管理 API 都在管理路径下，例如 `/manage/api/state`。根路径 `/api/state` 不提供管理接口。关闭伪装网站不影响管理、订阅和 ACME 验证。

统一域名后新生成的反代配置将整个站点转发到 PrismFlow **公开服务 8081**。8080 保留为本机管理/兼容入口；9097 是内核控制器，继续仅监听本机。

## 自定义 HTTPS 端口

统一访问地址可以填 `https://example.com:8443` 或 `https://example.com:10443`。管理路径、订阅链接、来源校验及部署检查均保留该端口；新建反代 TLS 的 XHTTP 也继承该端口，仍可单独修改。已有入口只在显式勾选同步时更新反代公网端口，直连协议的随机监听端口不改变。

Caddy 对应站点写为 `https://example.com:8443`，Nginx 使用 `listen 8443 ssl`；按实际部署同步 Docker 端口映射和防火墙放行规则。只改变服务器监听、不改变统一访问地址，会让生成的链接仍指向旧端口。若已设置 PRISMFLOW_ORIGIN，也应改成包含自定义端口的完整来源，例如 https://example.com:8443（不包含管理路径），并重启或重建服务以加载环境变量，否则会出现“来源不允许”。

网页使用 8443 不改变 CA 的验证端口要求：HTTP-01 仍只能使用公网 80；无法开放 80 时可使用 Cloudflare DNS-01。参见 [Let's Encrypt 验证方式](https://letsencrypt.org/docs/challenge-types/)。

## Caddy 部署

将以下站点合并到已有 Caddyfile，替换域名：

```caddy
example.com {
    reverse_proxy 127.0.0.1:8081
}
```

Caddy 自动管理网页 HTTPS；无需为管理和订阅各建一个站点。Caddy 与 PrismFlow 必须能访问同一宿主机回环地址：宿主机 Caddy 可以直接连接，容器 Caddy 可使用 Linux host 网络。独立 bridge 容器的 `127.0.0.1` 指容器自己，无法直接访问宿主机回环服务。

Nginx 使用同一 server，将 `location /` 原样反代到 `http://127.0.0.1:8081`，保留 Host 和 X-Forwarded-Proto，并配置原有 HTTPS 证书。面板中的「反代配置与检查」提供可复制的 Caddy/Nginx 配置。先合并、校验并重载反代，再在面板检查；PrismFlow 不直接改写反代文件。

若设置了 `PRISMFLOW_ORIGIN`，其值应为 `https://example.com`，不包含管理路径。它优先于面板保存的统一地址；通过 HTTPS 反代首次初始化前也应配置此变量，避免来源校验失败。

## 网页证书与协议证书

同一个域名可以被网页和代理协议共同使用，但证书由实际处理 TLS 的程序读取：

| 用途 | 证书处理方式 |
| --- | --- |
| 网站、管理、订阅、反代 TLS 的 XHTTP | Caddy/Nginx 的网页 HTTPS 证书 |
| AnyTLS、Hysteria2、TUIC、内核 TLS 等 | Mihomo 读取面板关联的协议证书 |
| VLESS REALITY | 无需自有证书；握手目标及 SNI 仍使用合适的外部站点 |

协议默认证书可以选择已有系统路径或自动申请。已有 Caddy/Nginx 证书可以通过只读挂载和正确的读取权限复用；证书与私钥路径须在 PrismFlow 进程/容器内可读，UID 10001 不会自动获得 root 文件的访问权限。Let's Encrypt live 符号链接还需要相应 archive 目录。

- **系统文件**：原工具续期；PrismFlow 每 30 秒检测文件变更，校验后自动应用到引用它的协议。读取或校验失败保留原运行证书。
- **acme.sh / 内置 ACME 自动申请**：默认使用正式 CA，启用自动续期。每 15 分钟检查，到期前 30 天尝试，同一证书尝试间隔至少 12 小时；成功后自动应用到引用它的协议，失败保留原证书并记录日志。
- **手动导入 PEM**：由管理员在到期前替换，不自动续期。

自动证书不能通过上传覆盖，以免丢失续期账户和验证信息。来源列只显示系统文件、手动导入或自动申请引擎；“测试 CA”属于状态，表示证书不受普通客户端信任。测试 CA 仅作为排查选项，默认关闭；已有测试证书可点击“申请正式证书”，通过正式验证后替换。

证书每行可打开操作记录，包含申请、续期、文件读取和失败原因；最近 200 条全局证书操作记录保存在本机数据库，重启仍保留。不会记录私钥或 DNS API 凭据，旧版本没有保存的记录无法补回。失败的新申请也会显示在列表中。

如果希望 Caddy/Nginx 直接使用 PrismFlow 申请的证书，需要另外配置共享文件和反代证书重载；本版不会自动改写或重载已有反代。默认建议由反代维护网页证书，PrismFlow 维护 Mihomo 的证书。

## 验证域名与申请证书

DNS A/AAAA 指向 VPS；同一个域名即可，不必先设置泛域名。若使用泛解析，它不会自动创建 HTTPS 站点或证书。不可达的 AAAA 可能使客户端连接失败。直连 REALITY/AnyTLS/Hysteria2 应使用 DNS-only 解析，不能经过普通 HTTP CDN 代理。

HTTP-01 需要将公网 TCP 80 的 `/.well-known/acme-challenge/` 转发到公开服务 8081；HTTPS 站点的转发不能代替 HTTP 验证路径。Cloudflare DNS-01 需要目标 Zone 的读取及 DNS 编辑权限。泛域名只能通过 DNS-01 申请，且客户端使用具体域名作为 SNI。

## 代理协议端口

网页共用 443 不等于所有代理协议共用 443：

| 协议 | 默认连接 |
| --- | --- |
| VLESS REALITY Vision | `example.com:随机五位端口/TCP` |
| VLESS XHTTP · 反代 TLS | `example.com:443`，根据独立路径转发到本机随机端口 |
| VLESS XHTTP · 内核 TLS、AnyTLS | `example.com:随机五位端口/TCP` |
| Hysteria2、TUIC | `example.com:随机五位端口/UDP` |

XHTTP 路径必须保留原样，不能与管理路径、订阅 `/s` 或 ACME 路径冲突；多个 XHTTP 路径也不能相同或互相包含。Nginx 必须开启 HTTP/2、关闭请求/响应缓冲。内核只在回环地址接收反代 XHTTP，公网订阅仍启用 TLS 校验。

REALITY 的握手目标是外部站点，不是自己的统一域名；它与订阅中用于连接 VPS 的域名有不同作用。

## 从旧的多域名部署迁移

1. 备份数据目录，保留 SQLite 与 master.key；更新 PrismFlow。
2. 通过原本机管理入口或旧管理域名，保存统一访问地址、管理路径和协议默认证书。先确认已有 `PRISMFLOW_ORIGIN` 是否与新地址一致。
3. 将统一站点的默认上游从 8080 改为 **8081**，保留需要的 XHTTP 独立路由，校验并重载 Caddy/Nginx。
4. 打开 `https://example.com/manage/` 登录，确认首页是伪装网站，订阅链接指向 `/s/令牌`，再执行入口检测和实际客户端验证。

旧的多域名设置仍可读取，已有协议不会在升级时被批量改写。全新安装也可先通过 SSH 隧道访问本机 8080 配置；公开站点配置完成后使用管理路径访问。
