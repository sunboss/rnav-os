# R导航 (rnav) 运维手册

> 2026-10-07 起由 sunboss 自行维护。Muse 不再代为改动容器。

## 架构速览

| 项 | 值 |
|---|---|
| 域名 | home.1234r.com |
| 容器名 | `rnav`（镜像 `rnav:latest`，**本地构建**，无公共镜像） |
| 后端 | Go（`service/`，Sun-Panel V2 兼容自研） |
| 前端 | Vue 3 + Vite（`web-src/`，构建产物在 `web/`） |
| 本地源码 | `~/workspace/rnav-os/` |
| 服务器目录 | `/data/sunboss/rnav/` |
| 端口 | 容器内 3002（经 Traefik 挂 home.1234r.com） |
| 数据 | `data/` 子目录：`conf/conf.ini`、`database.db`、`uploads/`、`files/` |

## 构建部署（改完代码后上线）

1. **构建前端**（改了 `web-src/` 才需要）：
   ```bash
   cd ~/workspace/rnav-os/web-src
   pnpm install
   pnpm run build   # 产物输出到 ../web/（vite.config.js 的 outDir）
   ```
2. **同步到服务器** `/data/sunboss/rnav/`（rsync 或 git，按你习惯来）。
3. **服务器上重构建并重启**：
   ```bash
   cd /data/sunboss/rnav
   docker compose up -d --build
   ```
   - 后端 `service/assets/bindata.go` 由 `Dockerfile.rnav` 构建阶段自动生成，
     含 go-bindata-assetfs 换行转义 bug 的 python 修复——**不要手改这个文件**。
   - 只改前端（`web/`）时，构建仍走同一条命令（镜像重打，约 1–2 分钟）。

## 常用操作

```bash
docker logs -f rnav                       # 看日志
docker compose up -d --build              # 重构建+重启（改代码后）
docker restart rnav                        # 单纯重启（改配置后，如 conf.ini）
curl -s https://home.1234r.com/api/health  # 健康检查，期望 db:"ok"
```

## 配置与数据（别弄丢）

- `data/conf/conf.ini`：R通行证 `client_secret` 已注入，**重装/覆盖时保留**。
- `data/database.db`：复用 Sun-Panel V2 结构，零迁移；改版前先备份：
  ```bash
  cp data/database.db data/database.db.bak.$(date +%F)
  ```
- 内网分组（id=11）已在前端隐藏；公开分组"R平台"（id=12）。

## 观察与排错

- `/api/health`：`db` 非 ok 或 503 → 查数据库文件权限/磁盘。
- `/debug/pprof/*`：仅管理员登录可进（访客回 1000）。
- 标准 MCP：`POST /mcp`（JSON-RPC，10 个工具）；WebMCP 试点：`webmcp.js`。
- Traefik 路由标签在 `docker-compose.rnav.yml`（生产规则已生效，如改域名更新 `rule=Host(...)`）。

## 回滚

镜像是本地构建、**无远端留档**。稳妥做法：
1. 改代码前 `git commit` 留点；
2. 出问题：`git checkout` 回上一版 → `docker compose up -d --build`；
3. 数据问题：用备份的 `database.db.bak.*` 覆盖恢复后 `docker restart rnav`。

## 已知事项（2026-10-07）

- 本地 `~/workspace/rnav-os/` 有未提交的改动（含 10-07 WebMCP 试点代码）；
  正式发版前先 `git commit`，别带着脏工作树部署。
- 前端 `Docker.vue` 有"状态"按钮（3 秒轮询容器 stats），依赖宿主机 docker.sock 直通，
  权限较大，后续可收紧（见 OPTIMIZATION_LOG.md OPT-014 后续项）。
