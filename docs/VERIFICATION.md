# PrismFlow 验收记录

日期：2026-10-29。开发环境为 Windows + PowerShell；Linux 验证使用 Docker Desktop。未连接生产 VPS，未修改公网 DNS；远程仓库为 github.com/ydfk/prismflow，默认分支 main；版本资源仅由 Tag 发布流程生成。

## 本轮创建流程

| 范围 | 结果 |
| --- | --- |
| 前端 | TypeScript + Vite 生产构建通过，静态资源嵌入 Go |
| 快速创建 | 四项互斥单选，每次新增一个；五位随机端口；切到普通创建保留协议、传输、域名、端口、证书、REALITY 目标、用户 |
| 普通创建 | 七种托管协议表单；原生类型列表与固定版本 listener/parse.go 的 20 种类型对齐 |
| 原生参数 | 单对象 YAML/JSON；阻止覆盖名称/类型/端口；需要实际内核校验，错误参数保存失败且 revision 不变，原核心继续运行 |
| 认证与脱敏 | Mixed 真实 HTTP 代理通信通过；错误密码被拒绝；原生参数不出现在状态轮询，不混入用户订阅，显式编辑鉴权并 no-store |
| 用户分配 | 创建和修改的入口/用户关联在同一事务；无效用户不能留下部分配置；原生入口不参加全选 |
| REALITY | 12 候选下拉、自定义目标、分批比较；TLS 1.3 / 有效证书 / h2 与公网目标限制的后端测试通过；浏览器比较结果使用模拟响应 |
| 导航与交互 | 首菜单“总览”；无重复工作台标题、无 P 头像；静态文字 caret 透明，输入控件保留光标 |
| 浏览器 | 3 项 Chromium E2E 全过，含订阅 YAML/二维码、原生失败回退、弹窗 Escape、历史 443 编辑、错误提示、手机无横向溢出及 API 隔离 |
| Go / Linux | 全量 Go 测试、vet；Linux 实际内核集成及 go test -race ./... 通过 |
| Docker | 最终镜像构建通过；UID 10001、AnyTLS 随机端口自动应用、订阅、重启持久化、令牌轮换、加密备份恢复通过 |
| 视觉复核 | 修后桌面和手机截图确认 radio 宽度/主色正常、协议名完整；独立 reviewer 对两项修复均判 resolved / ship；设计检测返回空列表 |

截图：docs/screenshots 下 overview.png、dark.png、light.png、mobile.png、protocols.png、mobile-protocols.png、ordinary.png、mobile-ordinary.png、native-listener.png。截图使用测试数据，图表没有伪造流量。

真实托管通信回归共 15 种：Shadowsocks TCP；Trojan、VMess、VLESS 各 TLS TCP/WS/gRPC；Hysteria2 QUIC、TUIC QUIC、AnyTLS TCP；VLESS REALITY+Vision；VLESS XHTTP TLS。TLS 测试只信任本次测试证书指纹，没有关闭校验。另有原生 Mixed 的真实通信与认证拒绝测试。

Windows 全量测试曾有一次 TUIC 返回 502，单独连续三次复测通过，之后全量复测通过；Linux race 全套通过。尚未确定该次瞬态失败的原因，未通过增加请求重试来掩盖它。

## 沿用的已有验证

- 内核锁定 v1.19.33。此前 Docker 实际调用 POST /api/core/install 完成官方下载、SHA-256、安装 /data/bin/mihomo、自动激活与重启沿用；本轮没有重复下载同一归档。
- 系统证书路径读取、域名与私钥校验、文件损坏时保留旧值、恢复后清除错误，以及状态 API 脱敏测试通过。
- 此前 Linux 实际下载 acme.sh 官方 3.1.6、校验 SHA-256、执行 --version、缓存复用通过。脚本引导不代表公网签发完成。
- 语法、实际端口占用、数据库提交失败、无效替换内核的失败回退测试保持通过。

## 外部验收边界

- 20 种原生监听器提供了参数入口，不代表全部已经逐一实网验收；本轮实际通信验证了 Mixed，其他类型仍依赖有效的原生参数、客户端和网络条件。
- TUN / Redirect / TProxy 需要目标系统的设备、权限与路由；非特权 Docker 不具备这些前置条件，未替用户配置宿主路由。
- 未在自有域名实际申请 Let's Encrypt 测试/正式证书，未执行真实 Cloudflare TXT 验证与跨天续期。
- Debian 12/13、Ubuntu 22.04/24.04 完整 systemd 安装/升级/卸载仍需目标系统验收；Linux amd64/arm64 归档为交叉编译，ARM64 硬件未运行验收。
- 真实公网 REALITY、Hysteria2 HTTP/3 网站回落、运营商 UDP、第三方客户端 URI 导入、长期负载及磁盘故障仍需实网/实机验证。

代理统计基于活动连接采样，可能漏短连接，不作计费依据。托管 Shadowsocks 仍为入口共享密码；原生监听器自行管理认证与客户端配置。
## 2026-10-29 字段说明、协议检测与服务器周期流量

- 4 项 Chromium E2E 通过：字段示例与帮助展开、网站归入设置和开关、关闭网站不影响订阅、订阅弹窗、取消/确认重置、用户名节点前缀、Header 上限与重置时间、原生检测 unknown、手机布局。
- 帮助说明改为正常文档流；验证说明底部不超过输入框顶部。Escape 先关帮助、再关弹窗。截图包含 settings.png、mobile-settings.png、subscription.png、field-help.png。
- 独立界面复核将帮助遮挡问题判定 resolved / ship；单次设计检测返回 []。skip-link 的 viewport 隐藏位置有断言，保留可访问入口。
- 服务器周期测试覆盖进程重启持久化、月末/闰年/时区、跨周期边界、计数归零/主机重启、网卡切换、读取不可用、仅出站 Header。Linux race 全套通过。
- 隔离客户端的真实 Shadowsocks 请求成功、错误密码失败、服务 PID 保持不变；15 种托管协议/传输通信回归通过。UI 对原生入口明确不判可用。尚未逐协议验证真实公网环境下的检测结果。
- 最终 Docker 镜像与生命周期验证通过，新增真实 /proc 网卡读取、Subscription-Userinfo 上限和 current 状态、未来重置时间检查。Docker 测试使用 bridge，所以统计的是容器接口；生产 Compose 使用 host 网络。
- 新周期监控从启用开始，不能反查启用前历史；周期边界的未观测间隙不追算。上限不做强制停用，Header 为服务器共享统计。Windows 明确显示 /proc 不可用。
- CA 实际签发、跨天续期、目标 systemd 与 ARM64 硬件等既有外部验收边界不变。
## 2026-10-30 发布与更新链路

- 新增 Tag-only Release 工作流；main/PR 只验证，Tag 必须位于 main 历史，发布两种架构的原生包、可加载 Docker 镜像、安装器与校验清单。版本从 Tag 注入主程序。
- 一键安装器支持交互选择和 --mode，稳定版/指定版本、架构识别、校验、数据保留与跨模式拦截。
- 一次性 root 容器使用本地 Release 夹具与 mock systemctl/docker：原生/Docker 安装、自动更新默认关闭、启停开关、成功升级、启动失败回退、校验损坏拒绝、root 状态目录保护与数据保留通过。此测试不等于真实 VPS 的 systemd 验收。
- 版本比较、预发布/草稿拒绝、GitHub 错误响应、版本检查缓存、非原生更新拒绝、管理员鉴权和 CSRF 有测试覆盖。
- GitHub 首次提交和 Actions 的实际运行状态以本轮最终结果及仓库为准；未推送 Tag 前不产生 Release，一键安装地址尚无发布资源。
## 2026-10-30 公网访问与 TLS 分工

- 三类地址分开：面板、订阅/网站、默认节点；基础域名用于填写建议。保存默认值不改变已有入口；设置 API 支持部分更新。
- XHTTP 外部 TLS：内核仅回环明文监听，订阅/URI 使用独立公网端口与 TLS 校验；既有内核 TLS、REALITY 等入口保持原行为。
- Caddy 2.6.2、Nginx 1.22.1 在隔离 Linux 容器内使用生成片段完成真实 TLS/XHTTP 请求；Nginx 开启 HTTP/2 并关闭请求与响应缓冲。非 root UID 1000 同样通过，证书仅信任本次测试指纹。HTTP-01 挑战片段另经 Caddy validate 校验。
- 泛域名系统证书可复用，显示 SAN、续期来源和节点关联；具体 SNI 与证书覆盖匹配。泛域名 HTTP-01 在发起任务前拒绝，DNS-01 分支可配置；未向真实 CA 申请证书。
- 公网检查验证 DNS、TLS 与面板/公开服务角色，拒绝重定向和错误角色；静态网站关闭不影响标识。没有改写宿主 Caddy/Nginx、DNS 或证书。
- TypeScript/Vite、Go vet、Windows Go 测试、Linux 全量 race 和真实内核协议回归通过。Chromium 6/6 E2E 通过，新增地址建议、保存前置、角色检查、快速转普通外部 TLS、手机无横向溢出；桌面/手机截图已更新。
- 独立视觉复核对唯一 P2（已保存配置的数据来源和保存前置提示）评分 resolved / ship；检测器本轮无发现；设计 tokens 未改变。
- 新 Docker 镜像构建与生命周期验收通过：UID 10001、自动应用、订阅/流量 Header、公开服务隔离、重启持久化、令牌重置及加密备份恢复。
- Linux amd64/arm64 原生归档已构建；本轮未重新验证 ARM64 实机或生产 systemd。公网域名的实际 DNS/CA 续期、现有证书权限及真实客户端入口仍需在部署 VPS 上检查。
## 2026-10-30 设置分组与品牌统一

系统设置改为服务管理、流量与记录、伪装网站、安全与备份四类；分类切换保留草稿并支持键盘导航。玻璃开关集中到外观浮层，重载后保持选择。平级主导航移除下拉箭头；页面和 favicon 使用同一个暖色 SVG，无独立图标副本或改色滤镜。

TypeScript/Vite 构建和 Chromium 7 项回归通过，覆盖分类显示、字段保存、草稿保留、键盘切换、玻璃偏好重载、图标来源一致性以及原有协议/订阅/域名/更新功能。桌面、手机、浅色主题截图已更新。此轮仅改前端与文档，不改变协议、证书或安装更新后端。
## 2026-10-30 Docker Hub 分发

Docker 发布改为 Tag 触发的 ydfk/prismflow 多架构镜像，GitHub Release 只保留原生包、安装器、Compose 与校验清单。仓库默认 Compose 拉取 Hub，源码构建需显式使用 compose.build.yaml。

Actions 语法与 Compose 配置解析通过。一次性 root 容器的 mock 安装测试覆盖 Hub 拉取、旧离线安装迁移、配置/数据保留、拉取失败不重建、Compose 镜像不匹配拒绝、启动命令失败恢复旧镜像、安装文件校验与预发布显式安装。真实 Hub 登录/推送、多架构公开清单和首次 Release 尚需配置 GitHub Secrets 并推送版本 Tag 后验证；本轮没有发布版本或上传镜像。
## 2026-10-30 精简发布流程与产物修复

按用户要求删除独立 CI，并从 Release 移除单元测试、race、协议探测和 Playwright 阶段，仅在版本 Tag 推送时构建发布。Docker 改为 linux/amd64，去掉 QEMU；原生包继续 amd64/arm64。下载阶段指定 name: native，构建阶段关闭 DOCKER_BUILD_RECORD_UPLOAD，避免将 .dockerbuild 记录误作 ZIP 解压。安装器在网络或系统变更前拒绝 ARM64 Docker 模式。

本地 Actions 语法、Shell 语法和安装脚本回归通过。未重跑旧 v0.1.0 任务，因为重跑仍绑定旧提交；修复的远程发布需使用包含修复的新 Tag。
