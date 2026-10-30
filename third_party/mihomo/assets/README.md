# 可选的离线构建输入

默认不提交二进制或源码归档。Docker 构建会先寻找以下本地文件，缺失时才从官方地址下载：

- amd64：`mihomo-linux-amd64-compatible-v1.19.33.gz`
- arm64：`mihomo-linux-arm64-v1.19.33.gz`
- 对应源码：`mihomo-v1.19.33-source.tar.gz`

内核压缩包来自 https://github.com/MetaCubeX/mihomo/releases/tag/v1.19.33 。
源码归档来自 https://codeload.github.com/MetaCubeX/mihomo/tar.gz/refs/tags/v1.19.33 。
只有从官方渠道取得的归档才应放入这里；内核始终验证 download.go 中固定的 SHA-256。
源码归档会与 GPL 许可证一同进入 Docker 镜像。
