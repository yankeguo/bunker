# bunker

一个简易的 Linux 堡垒机，提供 Web 管理界面，支持命令行和端口转发

## 安装

### 二进制

访问 [GitHub Releases](https://github.com/yankeguo/bunker/releases) 页面，并从最新的发行版下载二进制文件。

Web 界面静态资源已经嵌入到二进制文件中，无需额外下载。

- 准备一个 `data` 目录，里面放一个配置文件 `config.yaml`
- 运行 `bunker --data-dir data`

### 使用容器镜像

访问 [DockerHub Repository](https://hub.docker.com/repository/docker/yankeguo/bunker) 或者 [GitHub Packages](https://github.com/yankeguo?tab=packages&repo_name=bunker) 获取最新的容器镜像。

- 准备一个 `data` 目录，里面放一个配置文件 `config.yaml`
- 挂载 `/data` 并运行容器镜像, `docker run -p 8080:8080 -p 8022:8022 -v "$(pwd)/data:/data" yankeguo/bunker:latest`

## 初始化用户

在 `data` 目录中，额外存放一个 `users.yaml` 文件

```yaml
username: yanke
password: qwerty
is_admin: true
update_existing: true
---
username: guest
password: guest
```

## 配置文件

`config.yaml` 配置文件字段如下：

```yaml
ui: # for display only
  ssh_host: "my.fancy.domain"
  ssh_port: "8022"
server:
  listen: ":8080"
  # trust X-Forwarded-For / X-Forwarded-Proto from a front reverse proxy
  trust_proxy: false
ssh_server:
  listen: ":8022"
```

## SSH 主机密钥

Bunker 在首次连上目标服务器时记录它的主机密钥（每种算法一把）。如果已记录的密钥发生变化，后续连接会被拒绝。重装服务器后，可以在服务器管理页重置主机密钥。

## HTTP 服务

存活和就绪检查位于 `/debug/alive` 和 `/debug/ready`。性能分析和指标接口不会对外暴露。

## 从源码构建

环境要求：

- Go（最低版本见 `go.mod` 中的 `go` 指令）
- Node.js 22.19+（或 24.11+、26+）和 npm（用于构建 Web 界面）

Web 界面静态资源会被嵌入到二进制文件中，因此需要先生成 UI 再编译：

```bash
cd ui
npm ci
npm run generate
cd ..
go build -o bunker ./cmd/bunker
```

## 持续集成

测试和镜像构建会先生成 Web 界面（`ui/.output/public` 会被嵌入二进制文件）。发布的二进制来自 `./cmd/bunker`。同一套镜像标签会同时推到 GHCR 和 Docker Hub。

| 事件 | 会运行什么 |
| --- | --- |
| Pull request，或推送到 `main` 以外的分支 | `go test ./...` |
| 推送到 `main` | 同样的测试，然后发布 `ghcr.io/yankeguo/bunker:latest` 和 `yankeguo/bunker:latest` |
| 推送 semver 标签（`v1.2.3`、`v1.2.3-rc.1`，以及其他带 `-` 的预发布） | 同样的测试、semver 镜像标签，以及 GitHub Release |

| Git 标签 | 镜像标签 |
| --- | --- |
| `v1.2.3` | `1.2.3`、`1.2`、`1` |
| `v1.2.3-rc.1` | `1.2.3-rc.1` |
| `v0.2.0` | `0.2.0`、`0.2` |
| `v0.0.1` | `0.0.1` |

Docker 标签会去掉开头的 `v`。没有 commit SHA 标签。预发布后缀（`-rc`、`-beta`、`-alpha`，以及其他 semver 预发布）只发布完整版本。仅由前导零组成的浮动标签（`0`、`0.0`）不会发布。预发布标签会标成 GitHub pre-release，也不会成为仓库的 latest release。每个 GitHub Release 附带 `SHA256SUMS`，以及 Linux、macOS、Windows 在 amd64 和 arm64 上各一份压缩包（Windows 为 `.zip`，其余为 `.tar.gz`）。包内的二进制文件是 `bunker`（Windows 上是 `bunker.exe`）。

## 许可证

GUO YANKE, MIT License
