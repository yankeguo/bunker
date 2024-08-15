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
ssh_server:
  listen: ":8022"
```

## 许可证

GUO YANKE, MIT License
