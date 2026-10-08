# R导航 API 规范 (供前端使用)

后端: Go + gin, 基础路径 `/api`

## 公开接口（无需登录）

### 获取分组+卡片列表
- `POST /api/panel/itemCard/getGroupList`
- Header: 无需 token（访客模式自动过滤 `publicVisitHide=1` 的分组）
- 返回: `[{id, title, description, sort, publicVisitHide, cards: [{id, title, description, sort, cardData: {url, lanUrl, openMethod, icon: {src,text,type}, customUrls, textColor}}}]}]`
- openMethod: 1=当前页, 2=新标签页
- icon.type=3 表示 iconify 图标，`icon.text` 如 `devicon:google`

### R通行证登录
- `GET /api/rpass/auth-url` → `{authUrl, state}`
  - 前端跳转到 authUrl，用户在 R通行证 完成登录后会回到 `redirect_uri`（`https://home.1234r.com/auth/callback?code=...&state=...`）
- `POST /api/rpass/callback` body: `{code, state}` → 用户信息（含 token）
  - 成功后把返回的 `token` 存 localStorage，后续请求 Header 带 `token: <值>`

### 账号密码登录
- `POST /api/login` body: `{username, password}` → 用户信息（含 token）

## 需登录接口（Header: `token: <token>`）

### 卡片管理
- `POST /api/panel/itemCard/edit` — 新增/编辑卡片（有 id=编辑）
- `POST /api/panel/itemCard/deletes` — body `{ids: []}`
- `POST /api/panel/itemCard/saveSort` — body `[{id, sort}]`

### 分组管理
- `POST /api/panel/itemCardGroup/edit` — 新增/编辑（含 `publicVisitHide` 字段）
- `POST /api/panel/itemCardGroup/deletes` — body `{ids: []}`
- `POST /api/panel/itemCardGroup/saveSort`
- `POST /api/panel/itemCardGroup/setPublicVisitHide` — body `{id, publicVisitHide}`

### 退出
- `POST /api/logout`

## MCP 接口（Header: `Authorization: Bearer <mcp_token>`）
- `POST /api/mcp/listGroups`, `/api/mcp/listCards?group=xxx`
- `POST /api/mcp/addGroup` `{title, description, publicVisitHide}`
- `POST /api/mcp/addCard` `{title, url, group, description, icon, openMethod}`
- `POST /api/mcp/updateCard` `{card_id, ...}`, `/api/mcp/deleteCard` `{card_id}`

## 通用返回格式
成功: `{code: 0, msg: "...", data: ...}`
失败: `{code: 非0, msg: "错误信息"}`
token 失效: code=1001

## 前端要求
- 技术: Vue3 + Vite（已有 v1.3.0 前端在 `src/` 可参考，但 API 已变）
- 页面: 首页（分组+卡片展示）、登录页（含R通行证按钮）、管理后台（卡片/分组增删改、访客可见开关）
- 品牌: 标题"R导航"，logo 用 `/logo.png`（稍后提供）
- 性能: 首屏快、移动端适配
- 访客隐藏的分组（publicVisitHide=1）只有登录后可见
