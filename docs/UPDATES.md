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

安装器检测 amd64/arm64，下载对应 Release 资源并校验 SHA-256。原生安装需要 Debian 12/13、Ubuntu 22.04/24.04 和 systemd；Docker 模式可使用已安装的 Docker，在上述系统缺少 Docker 时会配置 Docker 官方 APT 仓库并安装 Engine/Compose 插件。

指定版本可追加 `--version v0.1.0`；原生模式可追加 `--auto-update`。默认选最新稳定 Release，不选择预发布。Release 尚未生成时会明确报错，不回退安装不确定的源码构建。

Docker 使用 Release 中的 `docker save` 镜像归档并 `docker load`，不依赖 GHCR 包的公开权限。数据在 `/opt/prismflow/data`，Compose 在 `/opt/prismflow/compose.yaml`；重新运行安装器保留这些文件，只切换镜像版本。常用命令需带当前镜像变量，例如：

```bash
cd /opt/prismflow
PRISMFLOW_IMAGE="$(cat image-version)" docker compose ps
PRISMFLOW_IMAGE="$(cat image-version)" docker compose logs -f prismflow
```

两种模式共用默认回环端口，安装器不会直接覆盖另一种已有部署；跨模式迁移应先备份并规划数据目录与端口。

## 原生版本更新

安装脚本部署独立 root 更新服务，面板本身仍以 prismflow 用户运行：

- `prismflow-update.path`：处理管理员在面板提交的更新请求。
- `prismflow-update.timer`：每小时触发检查，最多随机延迟 10 分钟。自动更新默认关闭，未开启时不访问版本接口。
- root 辅助脚本从固定仓库选择稳定 Release，校验归档、版本号并原子替换主程序；新进程启动失败时尝试恢复旧程序。
- root 状态目录 `/var/lib/prismflow-updater` 与面板可写的 requests 子目录分开；面板只能提交固定标记，不能指定下载 URL 或执行命令。

在「系统设置 → 面板版本与更新」检查版本、确认手动更新或开启自动更新。命令行等价操作：

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

默认分支为 main。main 推送与 PR 只验证，不产生发布包。只有推送合法版本 Tag 才运行 Release 工作流：

```bash
git tag v0.1.0
git push origin v0.1.0
```

Tag 必须落在 main 的历史中。支持 v主.次.补丁，以及 `v0.2.0-rc.1` 形式的预发布；预发布不会成为 latest，也不会进入自动更新。

所有资源先上传到草稿 Release，完成后才公开；失败可重跑草稿，已经公开的版本不会被覆盖。

发布内容：

| 文件 | 用途 |
| --- | --- |
| prismflow-linux-amd64.tar.gz / prismflow-linux-arm64.tar.gz | 带真实 Tag 版本的原生程序及安装脚本 |
| prismflow-docker-amd64.tar.gz / prismflow-docker-arm64.tar.gz | 对应架构的可加载 Docker 镜像 |
| install.sh | 可交互选择部署方式的一键安装器 |
| compose.release.yaml | 使用已加载镜像的生产 Compose 模板 |
| SHA256SUMS | 上述安装资源的 SHA-256 清单 |

首次 main 提交本身不会创建 Tag 或 Release。只有首个发布工作流完成后，一键命令才有可下载资源。GitHub Actions 的实际运行结果以仓库 Actions 为准，本地检查不等同于远程发布成功。
