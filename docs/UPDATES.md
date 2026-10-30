# 安装、版本发布与更新

## 一键安装

选择对应命令直接安装，无需交互选择。

**本地（原生）版本**

```bash
curl -fsSL https://github.com/ydfk/prismflow/releases/latest/download/install.sh | sudo bash -s -- --mode native
```

**Docker 版本**

```bash
curl -fsSL https://github.com/ydfk/prismflow/releases/latest/download/install.sh | sudo bash -s -- --mode docker
```

原生模式下载对应 amd64/arm64 Release 归档并校验 SHA-256，需要 Debian 12/13、Ubuntu 22.04/24.04 和 systemd。Docker 模式从 Docker Hub 拉取 `ydfk/prismflow`，Docker 根据宿主架构选择镜像；上述系统缺少 Docker 时会配置官方 APT 仓库并安装 Engine/Compose 插件。

指定版本可追加 `--version v1.2.3`；原生模式可追加 `--auto-update`。默认从最新稳定 GitHub Release 选择确切版本，不自动选择预发布；指定预发布 Tag 则允许安装对应版本。首次发布前没有可下载的安装器或可拉取的 Hub 镜像。

Docker 部署文件也从同一个 Release 获取并校验 SHA-256，镜像不再通过 GitHub 的 tar 归档分发。数据位于 `/opt/prismflow/data`，Compose 位于 `/opt/prismflow/compose.yaml`，`image-version` 记录完整镜像名。升级时先成功拉取镜像，再重建容器；拉取失败不改原部署，启动命令失败保留旧版本记录并尝试恢复旧镜像。Compose 返回启动命令成功不等于业务端到端验收，应继续查看日志和面板。

重新运行安装器保留用户的 Compose、环境配置和数据。旧 `docker load` 安装也可迁移，前提是 Compose 的 image 使用 `PRISMFLOW_IMAGE` 变量；若已硬编码其他镜像，安装器会报错并要求先手动调整。常用命令：

```bash
cd /opt/prismflow
export PRISMFLOW_IMAGE="$(cat image-version)"
docker compose ps
docker compose logs -f prismflow
```

两种模式共用默认回环端口，安装器不会直接覆盖另一种已有部署；跨模式迁移应先备份并规划数据目录与端口。

## Docker Hub 与手动 Compose

仓库根目录的 `compose.yaml` 默认拉取 [`ydfk/prismflow:latest`](https://hub.docker.com/r/ydfk/prismflow)。使用命名卷，容器以 UID 10001 运行；这与一键安装器的 bind 数据目录不同，不要直接混用。

```bash
docker compose pull
docker compose up -d
```

生产环境可在 `.env` 中设置完整镜像引用，例如 `PRISMFLOW_IMAGE=ydfk/prismflow:1.2.3`，版本请替换为已发布 Tag。默认 `latest` 跟随稳定版；升级固定版本时修改引用后重新 pull/up。

源码构建单独使用覆盖文件：

```bash
docker compose -f compose.yaml -f compose.build.yaml up -d --build
```

Release 中的 `compose.release.yaml` 默认固定当前发布版本，供一键安装器及手动 bind 部署使用；手动部署时需预先创建 data 目录并授予 UID 10001 写权限。证书挂载及 host 网络说明见 README 和 [域名部署指南](DOMAINS.md)。

## 原生版本更新

安装脚本部署独立 root 更新服务，面板本身仍以 prismflow 用户运行：

- `prismflow-update.path`：处理管理员在面板提交的更新请求。
- `prismflow-update.timer`：每小时触发检查，最多随机延迟 10 分钟。自动更新默认关闭，未开启时不访问版本接口。
- root 辅助脚本从固定仓库选择稳定 Release，校验归档、版本号并原子替换主程序；新进程启动失败时尝试恢复旧程序。
- root 状态目录 `/var/lib/prismflow-updater` 与面板可写的 requests 子目录分开；面板只能提交固定标记，不能指定下载 URL 或执行命令。

在「系统设置 → 服务管理 → 面板版本与更新」检查版本、确认手动更新或开启自动更新。命令行等价操作：

```bash
prismflow-update check
sudo prismflow-update now
sudo prismflow-update auto on
sudo prismflow-update auto off
sudo journalctl -u prismflow-update.service -n 100 --no-pager
```

更新面板会重启服务，管理会话和代理连接会中断，之后可能需要重新登录。配置、SQLite、证书、网站和数据目录中的 Mihomo 不被替换。自动更新不做面板降级，也不会自动追随 Mihomo 新版本。

旧安装未包含更新服务时，先运行新版安装器或安装包中的 install.sh；Docker 和直接源码运行不开放原生更新按钮。升级前建议导出备份并保留完整数据目录。

## Tag 发布流程

默认分支为 main。main 推送与 PR 只验证，不构建发布镜像或上传 Docker Hub。只有 main 历史中的合法版本 Tag 才运行 Release 工作流。

先在 GitHub 仓库 **Settings → Secrets and variables → Actions** 配置：

| Repository Secret | 内容 |
| --- | --- |
| `DOCKERHUB_USERNAME` | Docker Hub 登录用户，需要对 `ydfk/prismflow` 有推送权限 |
| `DOCKERHUB_TOKEN` | 该用户的 Docker Hub 读写访问令牌 |

与 caddy-pilot 使用相同的凭据名称，但不同仓库的 Repository Secrets 不自动共享。请在 Docker Hub 准备公开的 `ydfk/prismflow` 仓库，凭据只填写在 GitHub Secrets 中。随后推送版本 Tag：

```bash
git tag v1.2.3
git push origin v1.2.3
```

例子中的版本请替换为本次要发布的新版本。已公开的版本不允许重建覆盖，使用新的 Tag 发布变更。

| Git Tag | Docker Hub 标签 | GitHub Release |
| --- | --- | --- |
| `v1.2.3` | `1.2.3`、`1.2`、`1`、`latest` | 正式版 |
| `v1.3.0-rc.1` | `1.3.0-rc.1` | 预发布；不更新稳定别名 |

流程先校验、测试并构建双架构原生包，再通过 Buildx/QEMU 构建并推送 `linux/amd64,linux/arm64` 的完整版本镜像。GitHub 安装资源先上传草稿，完成后公开；只有正式 Release 公开后，独立任务才用同一镜像 digest 更新稳定别名。若仅别名更新失败，在 Actions 选择“重新运行失败的作业”，无需重建已公开版本。

GitHub Release 内容：

| 文件 | 用途 |
| --- | --- |
| prismflow-linux-amd64.tar.gz / prismflow-linux-arm64.tar.gz | 带真实 Tag 版本的原生程序及安装脚本 |
| install.sh | 一键安装器 |
| compose.release.yaml | 指向当前 Docker Hub 版本的生产 Compose 模板 |
| SHA256SUMS | 上述 GitHub 安装资源的 SHA-256 清单 |

Docker 镜像只分发到 Docker Hub，不再上传 prismflow-docker 架构 tar 包。面板内显示版本与 Tag 一致，例如 v1.2.3 对应 1.2.3。

发布实现参考 [caddy-pilot 的 Docker Hub 工作流](https://github.com/ydfk/caddy-pilot/blob/main/.github/workflows/docker-publish.yml)；多架构构建与清单别名使用 [Docker 官方 Actions 文档](https://docs.docker.com/build/ci/github-actions/multi-platform/) 和 [imagetools create](https://docs.docker.com/reference/cli/docker/buildx/imagetools/create/)。真实推送需要 GitHub Secrets 和版本 Tag，本地脚本/配置检查不等同于 Docker Hub 发布成功。
