# docker-proxy

[![License: MIT](LICENSE)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev/)

**仓库：** [github.com/Rehtt/docker-proxy](https://github.com/Rehtt/docker-proxy)

[English](#english) · [中文](#中文)

<a name="english"></a>

## English

**docker-proxy** is a lightweight reverse proxy for the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec) registry HTTP API (`/v2/...`). It routes traffic by the HTTP `Host` header so you can expose custom hostnames (for example `hub.example.com`, `ghcr.example.com`) and forward to real upstream registries (for example Docker Hub, GHCR).

**Repository:** [github.com/Rehtt/docker-proxy](https://github.com/Rehtt/docker-proxy)

Typical use cases: internal mirrors behind your own DNS/TLS names, or unifying multiple upstreams behind one ingress pattern.

**Note:** Docker Hub’s registry endpoint is `registry-1.docker.io`, not `hub.docker.com`.

---

<a name="中文"></a>

## 中文

### 简介

**docker-proxy** 是一个用 Go 编写的 **Docker / OCI 镜像仓库反向代理**。它根据客户端请求里的 **HTTP `Host` 头** 将 `/v2/...` 等 Registry API 请求转发到你在配置中指定的上游地址，从而可以用自定义域名（如 `hub.example.com`）访问官方仓库（如 Docker Hub、GHCR）。

适用于：内网统一入口、自建域名 + 证书、按域名区分上游来源等场景。

### 特性

- 基于 `Host` 的 **多路由**：不同域名指向不同上游 registry
- 使用标准库 `httputil.ReverseProxy`，流式传输，适合大层下载
- 支持 **HTTP / HTTPS** 监听（HTTPS 需自行提供证书）
- 提供 **Makefile**、**Dockerfile**、**docker-compose** 便于部署

### 架构说明

```
客户端 docker pull hub.example.com/...
        │
        ▼  Host: hub.example.com
   [ docker-proxy ]
        │
        ▼  Host 改写为上游，路径保持 /v2/...
   registry-1.docker.io (或其它 upstream)
```

代理会保留原始请求路径与查询串，仅将请求的 scheme/host 指向上游。上游需支持标准 Registry API。

### 环境要求

- Go 版本以 `go.mod` 中 `go` 指令为准（当前为 **1.24**）
- 访问上游 registry 的网络（通常需 HTTPS）

### 快速开始

#### 1. 配置

复制示例并编辑：

```bash
cp config.example.yaml config.yaml
```

`config.yaml` 示例：

```yaml
routes:
  - host: hub.example.com
    upstream: https://registry-1.docker.io
  - host: ghcr.example.com
    upstream: https://ghcr.io
```

- **`host`**：客户端访问时使用的域名（不含端口时仅匹配主机名；带端口时内部会去掉端口再匹配）。
- **`upstream`**：上游 registry 根 URL，必须为 `http://` 或 `https://`，且包含主机名。

> **Docker Hub**：拉镜像走的是 **`registry-1.docker.io`**，不是网站域名 `hub.docker.com`。将 `upstream` 写成错误地址会导致 pull 失败。

#### 2. 获取源码

```bash
git clone https://github.com/Rehtt/docker-proxy.git
cd docker-proxy
```

也可直接使用 Go 工具链安装（需已安装 Go）：

```bash
go install github.com/Rehtt/docker-proxy/cmd/docker-proxy@latest
```

#### 3. 本地运行

```bash
make build
./bin/docker-proxy -listen :8080 -config config.yaml
```

或使用：

```bash
go run ./cmd/docker-proxy -listen :8080 -config config.yaml
```

#### 4. Docker / Compose

```bash
make compose-up    # 需已存在 config.yaml
make compose-logs
make compose-down
```

镜像默认监听容器内 `8080`，配置文件挂载为只读：`./config.yaml` → `/etc/docker-proxy/config.yaml`。若使用下方缓存目录挂载，请在配置里将 `cache.dir` 设为容器内路径（例如 `/cache`）。

### 镜像缓存

对上游 Registry 的 **`GET`/`HEAD`**，且路径为 `/v2/.../blobs/...` 或 `/v2/.../manifests/...` 的响应，可按配置写入本地磁盘；**默认保留 3 天**（按文件 `meta` 的修改时间判断，到期后定期清理）。不缓存带 `Range` 的请求或 `Content-Encoding` 非 identity 的响应。

配置示例：

- **`enabled`**：`true` 启用，`false` 关闭。省略时：只要配置了非空 **`dir`** 即视为启用（兼容旧配置）。
- **`ttl_days`**：可省略；在已启用且未写天数时默认为 **3**。

```yaml
cache:
  enabled: true
  dir: ./cache
  ttl_days: 3
```

关闭缓存可写 `enabled: false`（可保留 `dir` 便于日后打开）。仅写 `cache: {}` 或不写 `cache` 也表示不启用。

命令行可覆盖：`-cache-dir`、`-cache-ttl-days`（`-1` 表示沿用配置文件中的天数）。

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-listen` | `:8080` | 监听地址，如 `:443`、`0.0.0.0:8080` |
| `-config` | `config.yaml` | 路由配置文件路径 |
| `-cert` | （空） | TLS 证书路径；与 `-key` 同时设置则启用 HTTPS |
| `-key` | （空） | TLS 私钥路径 |
| `-cache-dir` | （空） | 缓存根目录，非空则启用并覆盖配置中的 `cache.dir` |
| `-cache-ttl-days` | `-1` | 缓存保留天数；`-1` 表示使用配置；仅 `-cache-dir` 时默认 3 天 |
| `-log-level` | （空） | 日志等级，覆盖配置：`debug` / `info` / `warn` / `error` |

运行 `docker-proxy -h` 可查看内置帮助。

### Docker 客户端配置

客户端需要能解析你的自定义域名（DNS 或 `/etc/hosts`），并视情况配置 **insecure-registries**。

**HTTP（例如映射到本机 8080）** — 在 `/etc/docker/daemon.json` 中：

```json
{
  "insecure-registries": ["hub.example.com:8080", "ghcr.example.com:8080"]
}
```

然后执行：

```bash
sudo systemctl reload docker   # 或重启 Docker
docker pull hub.example.com:8080/library/nginx:latest
```

**HTTPS（443 且证书由系统信任）**：一般 **不需要** `insecure-registries`，可直接：

```bash
docker pull hub.example.com/library/nginx:latest
```

### Makefile 目标

| 目标 | 说明 |
|------|------|
| `make` / `make build` | 编译到 `bin/docker-proxy` |
| `make test` / `make vet` | 测试与静态检查 |
| `make run` | 编译并以 `config.yaml` 在 `:8080` 运行 |
| `make docker-build` | 构建镜像 `docker-proxy:latest` |
| `make docker-run` | 单容器运行并挂载当前目录 `config.yaml` |
| `make compose-up` / `compose-down` / `compose-logs` | Compose 启动 / 停止 / 日志 |

### 限制与说明

- 本工具做的是 **HTTP 反向代理**，不实现独立 registry 存储。
- 各上游的 **认证流程**（如 401、`WWW-Authenticate`）通常仍由客户端与上游 token 服务交互；若某 registry 在响应里写死了与域名相关的字段，可能需要额外扩展（当前未做响应头改写）。
- 生产环境建议使用 **HTTPS** 并正确配置防火墙与证书。

### 参与贡献

请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。安全相关披露见 [SECURITY.md](SECURITY.md)。

### 许可证

本项目以 [MIT License](LICENSE) 发布。
