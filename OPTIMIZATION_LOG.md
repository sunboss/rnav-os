# R导航优化存档（OPTIMIZATION LOG）

> **目的**：记录所有优化的背景、方法、效果和验证方式，让任何 AI（或人）能无缝继续优化工作。
> **规范版本**：v1.0（2026-09-29）
> **项目**：R导航（rnav-os）—— Sun-Panel V2 兼容自研版
> **源码**：`~/workspace/rnav-os/`

---

## 存档规范（其他 AI 继续优化时必须遵守）

每条优化记录必须包含以下字段：

| 字段 | 说明 |
|------|------|
| 编号 | OPT-XXX，按顺序递增 |
| 日期 | YYYY-MM-DD |
| 类别 | 前端/后端/数据库/部署/网络 |
| 背景 | 为什么要做这个优化（瓶颈在哪里） |
| 方法 | 具体怎么做的（文件、函数、关键代码） |
| 效果 | 量化数据（before → after） |
| 验证 | 怎么证明有效（测试方法、命令） |
| 风险 | 可能的副作用和回滚方式 |
| 后续 | 还可以继续优化的方向 |

---

## 已完成优化

### OPT-014：收尾优化全家桶（DSN/health/pprof/Docker stats/标准 MCP Server）— 2026-09-29
- **日期**：2026-09-29
- **类别**：后端正确性 + 可观测性 + Docker 管理器 + MCP 协议
- **背景**：P1/P2 已清完，用户要求"全部优化"。剩余 P3（UPX/Redis/pprof）中 UPX 收益极小、Redis 单实例不需要；转而补功能短板：缺健康检查、Docker 无实时状态、"MCP"只是 REST 接口非标准协议。
- **修改**：
  1. **busy_timeout 进 DSN**（`service/initialize/database/connect.go`）：之前 `db.Exec("PRAGMA busy_timeout=5000")` 只对建库时的首连接有效，连接池新连接 `busy_timeout=0`（实测确认）。改为 DSN `?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)`，每个新连接自动应用。正确性修复，非性能。
  2. **`/api/health`**（`service/router/A_ENTER.go`）：返回 `{code:0, data:{status, service:"rnav-os", db:"ok", time}}`，DB ping 失败回 503。容器自检/监控用，不鉴权。
  3. **pprof 仅管理员**（`service/router/A_ENTER.go`）：`router.Group("/debug/pprof", LoginInterceptor, AdminInterceptor)` + `gin.WrapH(http.DefaultServeMux)`。访客访问返回 1000（未登录）。
  4. **Docker 实时 stats**：后端 `POST /api/docker/container/stats`（`CoreDockerStats`，one-shot 非流式，CPU 用 Docker 官方算法 `cpuDelta/sysDelta*cpus*100`，内存 MB+百分比，网络累计 rx/tx）；前端 `Docker.vue` 新增"状态"按钮+弹窗，3 秒轮询，`onUnmounted` 清定时器。
  5. **Core 层抽离**：`docker.go` 的 Containers/Operate/Logs/Stats 抽为导出的 `CoreDockerContainers/CoreDockerOperate/CoreDockerLogs/CoreDockerStats`；`mcp.go` 抽为 `service/api/api_v1/mcp/core.go`（`coreListGroups/coreListCards/coreAddGroup/coreAddCard/coreUpdateCard/coreDeleteCard`），原 Handler 瘦身为薄封装。
  6. **标准 MCP Server**（`service/api/api_v1/mcp/server.go`）：`POST /mcp`，JSON-RPC 2.0（Streamable HTTP，当前 MCP 标准），Bearer 认证复用 `McpAuthInterceptor`。支持 `initialize`（protocolVersion 2024-11-05）、`notifications/initialized`（202）、`tools/list`、`tools/call`，共 10 个工具：rnav_list_groups/rnav_list_cards/rnav_add_group/rnav_add_card/rnav_update_card/rnav_delete_card/rnav_docker_containers/rnav_docker_operate/rnav_docker_logs/rnav_docker_stats。原有 REST 接口（`/api/mcp/*`）保持不变，mcp-gateway 零影响。
  7. **UPX 未做**：19MB→~6MB，服务器磁盘充足，省几 MB 无意义且增加启动解压耗时。apt 装 upx-ucl 曾卡住已放弃。如以后在意体积再做。
  8. **Redis 未部署**：代码层已支持（`cache_drive` 配置项），单实例内存缓存足够，多实例才需要。
- **验证**：`go build`/`go vet`/`gofmt` 全过；stats 解析用真实样本单测（CPU 40.0% 计算正确）；生产验证——`/api/health` 返回 db:ok；公网 API 8 分组 118 卡片（新增"R平台"分组+7 站点：R智能/R论坛/Meta论坛/R通行证/R算力/MCP控制中心/R导航）；JSON-RPC `initialize`+`tools/list`+`tools/call rnav_list_groups` 全通；访客访问 `/debug/pprof/` 被 1000 拒绝；前端 bundle 含 stats 代码。
- **风险**：低。DSN 改法经 mattn 驱动文档确认；MCP 新增端点不碰老接口；Docker stats 与 logs 同源调用模式。
- **后续**：受限 socket proxy（Docker 管理器仍直通宿主机 docker.sock，权限过高）；标准 MCP 的 SSE 传输（如有老客户端需要）；卡片上千时 `fetchList` 只 SELECT 必要字段。


### OPT-001：生产环境去黄条脚本性能优化
- **日期**：2026-09-29
- **类别**：前端（生产 hotfix）
- **背景**：生产 `home.1234r.com` 内联脚本用 `setInterval(800ms)` 全量扫描 DOM 去黄条，持续 15 秒，浪费 CPU。
- **方法**：改为 MutationObserver 精准捕获新增节点，命中即 disconnect；8 秒兜底扫描一次。
  - 源文件：`/tmp/rnav-gate-v2.js`
  - 已内联到生产容器 `/app/web/index.html`
- **效果**：DOM 扫描次数从 ~19 次全量扫描 → 1-2 次精准捕获
- **验证**：浏览器实测黄条消失、页面无错误
- **风险**：容器 recreate 后丢失，需重做（已知问题）
- **后续**：自研版前端应从源码层面移除黄条逻辑，不再需要运行时补丁

### OPT-002：后端公开分组列表缓存
- **日期**：2026-09-29
- **类别**：后端
- **背景**：`GetGroupList` 是最热路径（首页每次加载调用），每次都要查 DB + Preload 卡片 + 解析 JSON。
- **方法**：
  - 新增 `service/api/api_v1/panel/itemCardCache.go`
  - 公开列表缓存 5 分钟（`global.NewCache`，复用项目现有缓存基础设施）
  - 卡片/分组增删改/排序/显隐切换时主动失效（`clearPublicGroupsCache`）
  - 注意：models 包不能 import global（循环引用），缓存放在 api 层
- **效果**：缓存命中时 DB 查询从 2 次 → 0 次；响应时间从 ~50ms → ~5ms（预估，待压测）
- **验证**：`go build` 通过；待运行时验证缓存命中率
- **风险**：缓存失效遗漏会导致数据不一致；新增修改入口时必须调用 `clearPublicGroupsCache`
- **后续**：可加缓存命中率指标；考虑 Redis 分布式缓存（多实例时）

### OPT-003：API 响应 Gzip 压缩
- **日期**：2026-09-29
- **类别**：后端
- **背景**：分组列表 JSON 较大（111 张卡片），未压缩传输浪费带宽。
- **方法**：
  - 新增 `service/api/api_v1/middleware/GzipInterceptor.go`
  - 使用 `sync.Pool` 复用 gzip.Writer，避免每次分配
  - 跳过 `/assets/` 和 `/custom/`（静态资源由 CDN/反向代理处理）
  - 在 `router/A_ENTER.go` 注册为全局中间件
- **效果**：JSON 响应体积减少约 70-80%（预估，待实测）
- **验证**：`go build` 通过；待 curl 验证 `Content-Encoding: gzip`
- **风险**：极低；客户端不支持 gzip 时自动跳过
- **后续**：静态资源也可在 Nginx 层开启 gzip/brotli

### OPT-004：数据库索引优化
- **日期**：2026-09-29
- **类别**：数据库
- **背景**：`item_card` 按 `(user_id, item_card_group_id)` 查询，`item_card_group` 按 `user_id` + `public_visit_hide` 过滤，无索引时全表扫描。
- **方法**：
  - `models.EnsureItemCardIndexes()`：`idx_item_card_user_group(user_id, item_card_group_id)`、`idx_item_card_sort(sort)`
  - `models.EnsureItemCardGroupIndexes()`：`idx_item_card_group_user(user_id)`、`idx_item_card_group_hide(public_visit_hide)`
  - 在 `initialize/database/connect.go` 的 `CreateDatabase` 中调用（幂等，`IF NOT EXISTS`）
- **效果**：分组查询从全表扫描 → 索引扫描（111 张卡片时差异小，数据量大时显著）
- **验证**：`go build` 通过；可用 `EXPLAIN QUERY PLAN` 验证
- **风险**：极低；索引只加速读，写操作有轻微开销
- **后续**：监控慢查询，按需加索引

### OPT-005：二进制体积优化
- **日期**：2026-09-29
- **类别**：部署
- **背景**：Go 二进制 26MB，包含调试符号，镜像传输慢。
- **方法**：构建参数加 `-ldflags="-s -w"`（strip 符号表和 DWARF）
  - `Dockerfile.rnav` 第 19 行已更新
- **效果**：26MB → 19MB（-27%）
- **验证**：`ls -lh /tmp/rnav-opt-slim` 确认 19M；`go build` 通过
- **风险**：极低；panic 堆栈缺少行号（生产问题排查稍难）
- **后续**：可考虑 UPX 进一步压缩（约 -50%，但启动稍慢）

### OPT-006：公开用户信息缓存
- **日期**：2026-09-29
- **类别**：后端
- **背景**：`PublicModeInterceptor` 在每个公开请求都执行 `global.Db.First(&userInfo, "id=?", userId)` 查库；导航页访客流量大，这是每请求固定开销。
- **方法**：
  - `service/api/api_v1/middleware/PublicModeInterceptor.go` 新增 `publicUserCache`（10 分钟 TTL）
  - 缓存命中时直接返回，跳过 DB 查询
  - 提供 `ClearPublicUserCache()` 供公开用户变更时调用
  - 注意：系统设置 `GetValueByInterface` 本身已有缓存，无需重复优化
- **效果**：公开请求 DB 查询从 1 次 → 0 次（缓存命中时）
- **验证**：`go build` 通过；待运行时验证
- **风险**：公开用户变更后 10 分钟内可能读到旧数据；变更时需调 `ClearPublicUserCache`
- **后续**：公开用户设置变更处接入 `ClearPublicUserCache`

### OPT-007：批量排序事务化
- **日期**：2026-09-29
- **类别**：后端/数据库
- **背景**：`SaveSort` 对每个卡片/分组执行独立 UPDATE，每条都是独立事务，N 个卡片 = N 个事务开销。
- **方法**：
  - `service/api/api_v1/panel/itemCard.go` 的 `SaveSort`
  - `service/api/api_v1/panel/itemCardGroup.go` 的 `SaveSort`
  - 改为单事务批量更新，任一失败则 Rollback 并返回错误（之前是静默忽略错误）
- **效果**：N 个事务 → 1 个事务；错误从静默忽略 → 明确返回
- **验证**：`go build` 通过
- **风险**：极低；事务保证原子性，比之前更可靠
- **后续**：超大批量可考虑 CASE WHEN 单语句更新

### OPT-008：SPA fallback + logo 静态路由
- **日期**：2026-09-29
- **类别**：后端
- **背景**：前端子任务构建完成后发现两个问题：① `/login`、`/admin`、`/auth/callback` 直接访问会 404（R通行证 OAuth 回调直接挂掉）；② `/logo.png` 无静态路由返回 404。
- **方法**：`service/router/A_ENTER.go`
  - 加 `router.StaticFile("/logo.png", webPath+"/logo.png")`
  - 加 `router.NoRoute()`：非 `/api` 路径返回 `web/index.html`，`/api/*` 保持 JSON 404
- **效果**：前端路由直接访问/刷新正常；OAuth 回调可用
- **验证**：本地实测 `/login`、`/admin`、`/auth/callback` 均 200；`/logo.png` 200；`/api/nonexistent` 保持 JSON 404
- **风险**：极低
- **后续**：无

### OPT-009：Docker 管理器 API（unix socket 直调 Engine API）
- **日期**：2026-09-29
- **类别**：后端/新功能
- **背景**：用户明确要求自研版"保留 Docker 管理器"，此前代码完全缺失；V2 的 Docker 管理器是列表改卡片风格+右键启停/重启。
- **方法**：
  - 新增 `service/api/api_v1/panel/docker.go`：用 Go 标准库 `net/http` + `DialContext("unix", socket)` 直调 Docker Engine API v1.43，**不引入** docker client 大依赖（二进制体积不膨胀）
  - 配置：`service/initialize/config/config.go` 加 `[docker] socket_path`，默认 `/var/run/docker.sock`
  - 三个接口：`POST /api/docker/containers`（`GET /containers/json?all=1`）、`POST /api/docker/container/operate`（start/stop/restart）、`POST /api/docker/container/logs`（`GET /containers/{id}/logs`，自研 multiplexed stream 帧头解析，TTY 容器原始流原样返回）
  - 路由 `service/router/panel/docker.go`：`LoginInterceptor` + `AdminInterceptor` 双拦截，R通行证普通用户（role!=1）返回 1005
  - 安全：ID 只允许 12~64 位 hex（防路径穿越）；action 白名单仅 start/stop/restart；默认**不开放**删除容器/exec/创建容器；socket 不存在时返回 code -1 + 中文 msg，绝不崩溃；日志响应限 10MB
- **效果**：R导航获得与 V2 Docker 卡片右键菜单同等能力（列表/启停/重启/日志），且权限更严（V2 普通用户不能管理容器，本实现在后端强制）
- **验证**：`go build` 通过；本地实测：未登录→1000，普通用户→1005，管理员无 socket→中文干净错误，非法ID（`../../etc/passwd`）→拒绝，非法 action（delete）→拒绝；demux 单元测试（multiplexed/TTY/空/畸形帧）全部通过
- **风险**：低。Engine API 版本固定 v1.43，老版本 Docker（API <1.43）可能字段缺失；socket 需部署时挂载 `-v /var/run/docker.sock:/var/run/docker.sock`
- **后续**：前端 Docker 卡片页对接（API 契约见 `API_DOCKER.md`）；后续可加容器 stats（实时网速/流量，对标 V2 2x2 Docker 卡片）

### OPT-010：cardSize 透传确认 + description 访客过滤
- **日期**：2026-09-29
- **类别**：后端/兼容性+安全
- **背景**：生产库 125 张卡片 card_size=2（标准尺寸），必须原样返回不改变外观；V2 特性要求卡片备注仅登录可见。
- **方法**：
  - 确认 `models.ItemCard.CardSize` 的 json tag 为 `cardSize`，`GetGroupList` 经 `Preload("Cards")` 直接序列化，字段天然透传，无需改动
  - `service/api/api_v1/panel/itemCard.go` 的 `GetGroupList`：`publicOnly`（访客）时将每张卡片的 `Description` 置空后再返回；公开缓存仅服务访客路径，缓存副本同样被清空是安全的（幂等）
- **效果**：访客看到的卡片无备注，登录用户正常返回；card_size=2 继续渲染为当前标准尺寸
- **验证**：本地实测访客 `getGroupList` → `cardSize:2`、`description:""`；管理员登录后 → `cardSize:2`、`description` 正常返回
- **风险**：极低
- **后续**：前端按 cardSize 做多尺寸混排时（V2 的 1x1/2x2 等），后端字段已就绪

### OPT-011：隐藏分组仅管理员可见 + 写接口管理员鉴权（安全收紧）
- **日期**：2026-09-29
- **类别**：后端/前端/安全
- **背景**：用户要求"内网内容必须管理员登录才可见"。原逻辑：隐藏分组（public_visit_hide=1）对所有登录用户可见；卡片/分组写接口仅验登录，R通行证普通用户（role=2）可增删改卡片、开关隐藏，属越权漏洞。
- **方法**：
  - `service/api/api_v1/panel/itemCard.go` 的 `GetGroupList` 重写权限判定：
    - `isAdmin = !isPublicVisit && userInfo.Role == 1`（访客模式下 userInfo 是公开账号本身，必须排除访客模式，不能只看 Role）
    - 内容归属固定为面板所有者（`panel_public_user_id`，未配置时回退当前用户）：访客/普通用户/管理员看同一套面板，R通行证新用户不再看到空面板
    - `publicOnly = !isAdmin`：访客走公开缓存+备注清空；普通登录用户直查DB（公开分组，备注保留）；管理员直查DB（全部分组）
  - `service/router/panel/itemCard.go`：写接口组加 `middleware.AdminInterceptor`（卡片/分组增删改、排序、setPublicVisitHide），非管理员返回 1005
  - 前端 `web-src/src/api.js` 新增 `isAdmin()`；`router.js` 拦截非管理员进 `/admin`、`/admin/docker`（回首页，后端同样 1005 拦截）；`App.vue` 仅管理员显示"管理"入口；`Home.vue` 隐藏徽标"需登录"→"仅管理员"；`Admin.vue` 文案同步
- **效果**：三档权限——访客（公开分组，无备注）、普通用户（公开分组，有备注）、管理员（全部分组+管理后台+Docker）
- **验证**：本地三档实测——访客只见"常用"无备注；普通用户只见"常用"有备注；管理员见"常用"+"内网"；普通用户调 edit/deletes/setPublicVisitHide/docker 全部 1005；管理员 edit 成功（code 0）；数据未被篡改
- **风险**：低。注意 `getCachedGroupList` 的访客缓存复用 ownerId 做 key，与之前访客路径的 key（公开账号ID）一致，缓存语义不变

### OPT-012：OAuth 加固 + SQLite 并发优化 + 静态缓存策略 + sync.Once（安全/性能）
- **日期**：2026-09-29
- **类别**：后端/数据库/网络/安全
- **背景**：部署前安全审查发现 4 类问题：① R通行证回调每次新建 `http.Client`（连接无法复用）；② 回调 JSON 无请求体大小限制；③ 上游响应 `io.ReadAll` 无上限；④ 内部错误原文回传客户端（信息泄漏）。另有性能项：SQLite 默认 journal 模式并发差、无 busy_timeout；高频查询缺复合索引；静态资源无 Cache-Control；两处缓存懒初始化存在并发竞态。
- **方法**：
  1. **OAuth 加固**（`service/api/api_v1/system/rpass.go`）：包级复用 `rpassHTTPClient`（15s 超时）；回调 `http.MaxBytesReader` 限 32KB；`code`≤512、`state`≤64 字符；上游响应 `io.LimitReader` 限 1MB；显式校验上游 HTTP 200；内部错误只 `global.Logger.Warnf` 记日志，客户端统一返回通用文案（不拼接 `err.Error()`）。state 单次使用+10分钟过期此前已实现（OPT-009），本次保留。
  2. **SQLite 并发**（`service/initialize/database/connect.go` 的 `SQLiteConfig.Connect`）：启动时执行 `PRAGMA journal_mode=WAL`、`busy_timeout=5000`、`synchronous=NORMAL`；连接池 `SetMaxOpenConns(10)`/`SetMaxIdleConns(5)`。
  3. **复合索引**（`service/models/itemCard.go`）：新增 `idx_item_card_group_sort ON item_card(item_card_group_id, sort)`（覆盖 Preload 热查询）、`idx_item_card_group_user_hide_sort ON item_card_group(user_id, public_visit_hide, sort)`（覆盖 GetList 主查询）、`EnsureUserIndexes` 建 `idx_user_rpass_id ON user(rpass_id)`（R通行证登录查询；`connect.go` 的 `CreateDatabase` 中调用）。
  4. **静态缓存**（`service/router/A_ENTER.go`）：WEB 服务块前加中间件——`/assets/*`（Vite content hash 产物）`Cache-Control: public, max-age=31536000, immutable`；其余（`/`、SPA fallback、图标、上传文件）`no-cache`。中间件只作用于其后注册的路由，API 路由不受影响。
  5. **sync.Once**：`service/api/api_v1/panel/itemCardCache.go`（`publicGroupsCacheOnce`）和 `service/api/api_v1/middleware/PublicModeInterceptor.go`（`publicUserCacheOnce`）的懒初始化由 `if nil` 改为 `sync.Once`，消除并发重复创建竞态。
- **效果**：回调接口抗超大请求体/超大上游响应；错误信息不再泄漏内部细节；SQLite 读并发提升、锁冲突等待 5s 而非直接报错；高频查询走覆盖索引；静态资源二次访问 0 请求（immutable），发版后 index.html 即时失效。
- **验证**：`go build ./...` 通过；本地起服实测——`/` 与 `/admin` 返回 `Cache-Control: no-cache`，`/assets/index-*.js` 返回 `immutable`；新库 `PRAGMA journal_mode=wal`、`synchronous=2(NORMAL)`，3 个新复合索引已创建；70KB 超大回调请求体被拒（参数错误）；管理员登录+`getGroupList` 返回 code 0；未登录调 Docker API 被拒；`rpass/auth-url` 正常返回授权地址。测试进程已清理。
- **风险**：低。WAL 模式会在 DB 目录产生 `-wal`/`-shm` 附加文件，备份数据库时需连同拷贝或先 checkpoint；`immutable` 依赖 Vite 文件名 hash，若未来改构建工具去 hash 需同步调整。
- **后续**：P1 的 `ParseCardData` 延迟解析仍可做；`busy_timeout` 是按连接设置，多实例部署时考虑 Redis 分布式缓存（P3）。

### OPT-013：card_data 延迟解析（性能）
- **日期**：2026-09-29
- **类别**：后端/性能
- **背景**：`GetGroupList` 等列表接口每次查询后逐张 `json.Unmarshal` 解析 `card_data`（111 张卡全量解析），读多写少场景下浪费 CPU。P1 待办。
- **方法**（`service/models/itemCard.go`）：
  - `ItemCard` 新增未导出 `cardDataParsed bool` 标记；`ParseCardData()` 改为幂等（已解析直接返回）。
  - 删除 `GetByUserId`、`fetchList` 中的 eager 解析循环；`panel/itemCard.go` 的创建/更新回显删除冗余 `ParseCardData()`。
  - 新增值接收者 `MarshalJSON()`：序列化时才解析。必须用值接收者——API 层 `SuccessData(c, req)` 以值类型序列化，指针接收者对 interface{} 中的值不会被调用（实测陷阱）。
  - `PackCardData()` 先 `ParseCardData()` 再打包：防止"DB 读出未解析就打包"时用零值覆盖 `card_data`。
  - 保留两处显式解析：`mcp.go` 列表（直接读 `ParsedCardData` 字段组装自定义结构，MarshalJSON 覆盖不到）、`mcp.go` UpdateCard（先解析→改值→打包的变更语义）。
- **效果**：列表查询省掉 N 次 Unmarshal，只在真正序列化输出时解析；实测 1 张卡片的创建→列表→更新全流程 cardData 完整。
- **验证**：`go build ./...` 通过；本地起服实测——创建卡片回显 `cardData.url` 正确；`getGroupList` 返回完整 `cardData`（url+icon）；更新标题后 DB 中 `card_data` 原始 JSON 未被破坏；测试进程已清理。
- **风险**：低。内存缓存（gocache）直接存 Go 值不经过 JSON，不受影响；`PackCardData` 的自愈解析保证老调用路径安全。
- **后续**：如卡片量上千，可考虑在 `fetchList` 中只 SELECT 需要字段，`card_data` 大文本列延迟加载。

---

## 待优化清单（按优先级）

### 高优先级
- [x] **P1**：前端构建产物瘦身——自研前端已替换（JS ~118KB），原 130KB vendor 为 Sun-Panel 官方包
- [x] **P1**：`ParseCardData` 每次请求解析 JSON——OPT-013 已完成（MarshalJSON 延迟解析）

### 中优先级
- [x] **P2**：SQLite WAL 模式（并发读优化）——OPT-012 已完成（WAL + busy_timeout=5000 + synchronous=NORMAL + 连接池限制）
- [x] **P2**：静态资源 Cache-Control 头（`web/` 下的 JS/CSS 加长缓存）——OPT-012 已完成（/assets/* immutable 1年，index.html/SPA fallback no-cache）
- [x] **P2**：缓存懒初始化改 `sync.Once`——OPT-012 已完成（publicGroupsCache、publicUserCache 两处）

### 低优先级
- [x] **P3**：UPX 压缩二进制——OPT-014 评估后放弃（19MB 体积无优化必要，见 OPT-014.7）
- [x] **P3**：Redis 分布式缓存——OPT-014 确认代码层已支持（cache_drive），单实例不部署
- [x] **P3**：pprof 性能分析端点——OPT-014 已完成（/debug/pprof/*，Login+Admin 双拦截）

---

## 性能基线（2026-09-29）

| 指标 | 数值 | 备注 |
|------|------|------|
| 首页 HTML | ~60ms | 生产实测，Cloudflare HIT |
| 二进制体积 | 19MB | `-s -w` 后 |
| vendor bundle | ~130KB | Sun-Panel 官方，待自研前端替换 |
| 卡片数量 | 111 | 生产数据 |
| 分组数量 | 8 | 生产数据 |

## 本地联调验证（2026-09-29）

后端 + 前端本地联调全部通过：

| 测试项 | 结果 |
|--------|------|
| 首页服务 | 200 正常 |
| 访客 getGroupList | 只返回公开分组，隐藏分组被过滤 ✅ |
| 登录用户 getGroupList | 返回所有分组（含隐藏）✅ |
| Gzip 压缩 | 804字节 → 383字节 (-52%) ✅ |
| 公开列表缓存 | 二次请求 <1ms ✅ |
| 缓存失效 | 添加卡片后访客列表立即更新 ✅ |
| 账号密码登录 | 正常（默认密码 12345678）✅ |
| 前端构建 | 无黄条文本，API 路径与后端路由完全匹配 ✅ |

**注意**：Docker 镜像构建需在服务器上执行（本地无 Docker），待用户确认后进行。

---

## 变更历史

- 2026-09-29：创建文档，记录 OPT-001 ~ OPT-005
- 2026-09-29：新增 OPT-006（公开用户信息缓存）、OPT-007（批量排序事务化）
- 2026-09-29：新增 OPT-008（SPA fallback + logo 静态路由）
- 2026-09-29：新增 OPT-009（Docker 管理器 API）、OPT-010（cardSize 透传 + description 访客过滤）
- 2026-09-29：新增 OPT-011（隐藏分组仅管理员可见 + 写接口管理员鉴权）
- 2026-09-29：新增 OPT-014（DSN/health/pprof/Docker stats/标准 MCP Server/R平台卡片），P1/P2/P3 清单全部关闭
