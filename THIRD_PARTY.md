# 第三方组件与参考

PrismFlow 的原创程序、界面和 Logo 使用 MIT，见 LICENSE。

## 独立代理内核

Mihomo 通过独立子进程和本机 HTTP API 运行，未链接进 PrismFlow Go 程序。
内核有自己的 GPL-3.0 许可证，不被 PrismFlow 的 MIT 许可证覆盖。

- 项目：https://github.com/MetaCubeX/mihomo
- 固定版本：https://github.com/MetaCubeX/mihomo/tree/v1.19.33
- 原始二进制：https://github.com/MetaCubeX/mihomo/releases/tag/v1.19.33
- 对应源代码归档：https://github.com/MetaCubeX/mihomo/archive/refs/tags/v1.19.33.tar.gz
- 完整许可证：third_party/mihomo/LICENSE
- 下载 SHA-256 固定在 internal/core/download.go。

Linux 发布包只包含 PrismFlow；安装脚本按需下载独立 Mihomo。
Docker 镜像不包含 Mihomo 二进制或源码归档；首次启动缺少内核时，从上述官方发行页下载并校验 SHA-256，保存到持久化数据目录的 bin/mihomo。其许可证与对应源码可通过上述上游链接获取。

## 程序依赖

直接依赖版本以 go.mod / go.sum 和 web/package-lock.json 为准。

| 组件 | 用途 | 上游许可证 |
| --- | --- | --- |
| modernc.org/sqlite | 无 CGO 的 SQLite 驱动 | BSD-3-Clause；SQLite 公有领域 |
| gopkg.in/yaml.v3 | YAML 编解码 | MIT / Apache-2.0 |
| golang.org/x/crypto | bcrypt、scrypt | BSD-3-Clause |
| go-acme/lego | ACME 证书签发 | MIT |
| React / React DOM | 用户界面 | MIT |
| Lucide | 图标 | ISC |
| qrcode | 本地二维码生成 | MIT |
| Manrope | 本地字体 | SIL OFL-1.1 |

运行依赖、Go 标准库和字体的原始许可证汇总在 `third_party/dependencies.txt`，随发布包和镜像一起提供。更新依赖后运行 `node scripts/collect-licenses.mjs` 重新生成。

## 设计和功能参考

- mack-a/v2ray-agent：参考多协议部署、证书和订阅管理流程，未复制其脚本代码。
- nesdesignco/Glassmorphism-Admin-Panel-UI：参考玻璃材质与管理布局，未复制源码或图片。
- 协议配置以 Mihomo 官方文档和 v1.19.33 入站定义为依据。

## acme.sh

面板可按需下载官方 acme.sh 3.1.6，通过独立 shell 子进程调用，不链接进 Go 程序。原项目及其许可证保留于下载的工具目录。

- 项目：https://github.com/acmesh-official/acme.sh
- 版本：https://github.com/acmesh-official/acme.sh/tree/3.1.6
- 官方源码归档：https://codeload.github.com/acmesh-official/acme.sh/tar.gz/refs/tags/3.1.6
- 固定归档 SHA-256：0d3f9000ac44a6331314742a88c475f79134e24fc991997883652adc59efc486
- 上游许可证 GPL-3.0，不受 PrismFlow MIT 覆盖。Docker 增加 curl、openssl 等发行版组件，保留系统自带许可证。
