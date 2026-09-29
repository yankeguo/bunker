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
  # trust X-Forwarded-For / X-Forwarded-Proto from a front reverse proxy
  trust_proxy: false
ssh_server:
  listen: ":8022"
```

## SSH host keys

On the first successful connection to a target, Bunker pins that server's host key (one key per algorithm). A later connection is refused if a recorded key changes. After reinstalling a server, reset its host key from the server page.

## HTTP service

Liveness and readiness are served at `/debug/alive` and `/debug/ready`. Profiling and metrics endpoints are not exposed.

## Build from Source

Requirements:

- Go (see the `go` directive in `go.mod` for the minimum version)
- Node.js 22.19+ (or 24.11+, or 26+) and npm (for the web UI)

The web UI assets are embedded into the binary, so the UI must be generated before compiling:

```bash
cd ui
npm ci
npm run generate
cd ..
go build -o bunker ./cmd/bunker
```

## Continuous integration

Tests and image builds generate the web UI first (`ui/.output/public` is embedded into the binary). The release binary is `./cmd/bunker`. The same image tags are pushed to GHCR and Docker Hub.

| Event | What runs |
| --- | --- |
| Pull request, or a push to any branch other than `main` | `go test ./...` |
| Push to `main` | the same tests, then `ghcr.io/yankeguo/bunker:latest` and `yankeguo/bunker:latest` |
| Push of a semver tag (`v1.2.3`, `v1.2.3-rc.1`, and any other `-` pre-release) | the same tests, semver image tags, and a GitHub Release |

| Git tag | Image tags |
| --- | --- |
| `v1.2.3` | `1.2.3`, `1.2`, `1` |
| `v1.2.3-rc.1` | `1.2.3-rc.1` |
| `v0.2.0` | `0.2.0`, `0.2` |
| `v0.0.1` | `0.0.1` |

Docker tags drop the leading `v`. There is no commit-SHA tag. Pre-release suffixes (`-rc`, `-beta`, `-alpha`, and any other semver pre-release) publish the full version only. Floating tags that would be only a leading zero (`0`, `0.0`) are not published. A pre-release tag is marked as a GitHub pre-release and is not made the repository's latest release. Each GitHub Release attaches `SHA256SUMS` and an archive per mainstream OS and architecture: Linux, macOS, and Windows, on amd64 and arm64 (`.tar.gz`, or `.zip` on Windows). The binary inside is `bunker` (`bunker.exe` on Windows).

## Credits

GUO YANKE, MIT License
