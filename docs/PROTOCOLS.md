# 协议创建与监听器

固定内核：Mihomo v1.19.33。

## 快速创建

| 方案 | 传输 | 证书 | 用户身份 |
| --- | --- | --- | --- |
| VLESS + REALITY + Vision | TCP | 无需自有证书，需 REALITY 握手目标 | 独立 UUID |
| VLESS + XHTTP + TLS | 服务端 auto / 客户端 packet-up | 反代维护，或绑定内核证书 | 独立 UUID |
| AnyTLS | TCP | 必须 | 独立密码 |
| Hysteria2 | QUIC / UDP | 必须 | 独立密码 |

每次选择一种，不会同时创建多个入口。「转为普通创建」保留协议、域名、五位端口、证书、REALITY 目标及所选用户，可以继续修改传输、路径、SNI、名称等参数。

## 普通创建

表单模式支持 VLESS、VMess、Trojan、AnyTLS、Hysteria2、TUIC v5、Shadowsocks。传输和 TLS/REALITY 选项根据所选协议变化，REALITY 的表单组合为 TCP + Vision；Shadowsocks 表单采用 chacha20-ietf-poly1305，共享入口密码。表单入口提供用户分配及 Mihomo YAML、分享链接、二维码订阅。

原生参数模式覆盖锁定版 `listener/parse.go` 的 20 种监听器：VLESS、VMess、Trojan、AnyTLS、Hysteria2、TUIC、Shadowsocks、SOCKS、HTTP、Mixed、Snell、Mieru、Sudoku、ShadowQUIC、Hysteria2 Realm、TrustTunnel、Tunnel、Redirect、TProxy、TUN。包括常用协议在内，均可切换到原生参数模式编辑更多内核选项。

原生编辑器接受单个 YAML/JSON 对象，不包含 `name`、`type`、`port`：这三个字段由表单管理，其他参数原样传给内核。认证、TLS、证书路径等在原生对象中配置；默认 `listen: 127.0.0.1`，需要公网访问时自行修改监听地址。原生入口不分配订阅用户，也不自动生成客户端配置，避免导出与原生认证不一致的凭据。参数只在显式编辑时读取，不出现在状态轮询中。

原生配置保存前需要已安装的实际 Mihomo 校验，启用时还会等待监听器就绪；失败会保持原配置。内核语法检查不保证系统权限、网络和路由可用。TProxy、Redirect、TUN 依赖宿主系统路由、设备和权限，普通非特权 Docker 容器不具备这些条件。TUN 不占用 TCP/UDP 监听端口。

所有新端口随机在 10000–65535 范围生成，避开已配置和本机占用。普通创建可修改生成值，历史入口编辑可保留原端口。公网访问与默认证书在证书页设置；新建自动继承，可单独覆盖，保存默认值不改变现有节点。生成端口不会替你开放云防火墙。外部 TLS 的 XHTTP 仅监听 127.0.0.1，订阅导出独立公网端口；其他托管入口监听 0.0.0.0，原生模式自行指定。反代与证书分工见 [域名部署说明](DOMAINS.md)；多个 TCP 协议共用一个端口的分流层尚未实现。

## REALITY 目标

下拉提供 Microsoft、Apple、Cloudflare、Google、Bing、Amazon、Samsung、NVIDIA、Mozilla、Wikipedia 等 12 个候选，并可选择「自定义目标」。候选只是便捷输入，不保证任何特定地区可用。

「检测当前」与「比较候选」从服务器检查公网目标的 TLS 1.3、证书与主机名校验、h2 协商和握手延迟。比较按每批四个目标执行并排序。选择通过且延迟较低者后，仍需导入客户端验证完整 REALITY 连接。

## 依据

- [固定版本监听器清单](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/listener/parse.go)
- [Mihomo 监听器参数](https://wiki.metacubex.one/config/inbound/listeners/)
- [固定版本 VLESS 入站源码](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/listener/inbound/vless.go)
## 检测和订阅名称

协议表格提供分步检测：用实际订阅参数运行临时客户端完成请求，无直连回退，不改变在线内核。原生监听器暂不自动生成检测所需客户端配置。订阅节点统一使用「用户名 · 入口名」。更多边界见 [使用说明](USAGE.md)。
