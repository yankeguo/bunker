# bunker

A simple bastion system for linux hosts

[中文文档](README.zh.md)

## Installation

### From Binary

Visit [GitHub Releases](https://github.com/yankeguo/bunker/releases) and download the latest release.

Static assets are embedded in the binary, so you don't need to download anything else.

- Prepare a `data` directory and put `config.yaml` configuration file in it
- Run `bunker --data-dir data`

### From Container Image

Visit [DockerHub Repository](https://hub.docker.com/repository/docker/yankeguo/bunker) or [GitHub Packages](https://github.com/yankeguo?tab=packages&repo_name=bunker) for container images

- Prepare a `data` directory and put `config.yaml` configuration file in it
- Run container image with `/data` mounted, `docker run -p 8080:8080 -p 8022:8022 -v $PWD/data:/data yankeguo/bunker:latest`

## Initial Users

Put a `users.yaml` file in `data-dir` to initialize the system with users.

```yaml
username: yanke
password: qwerty
is_admin: true
update_existing: true
---
username: guest
password: guest
```

## Configuration File

Prepare a `config.yaml` file

```yaml
ui: # for display only
  ssh_host: "my.fancy.domain"
  ssh_port: "8022"
server:
  listen: ":8080"
ssh_server:
  listen: ":8022"
```

## Credits

GUO YANKE, MIT License
