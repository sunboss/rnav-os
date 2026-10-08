# R导航 前端 (web-src/)

Vue3 + Vite 轻量前端，从零编写（未使用 v1.3.0 臃肿前端）。

## 页面

- `/` 首页：分组+卡片展示，访客自动过滤隐藏分组；icon.type=3 用 iconify CDN 直链 SVG 渲染
- `/login` 登录：账号密码 + R通行证登录按钮
- `/auth/callback` R通行证 OAuth 回调（code/state 换 token）
- `/admin` 管理页（需登录）：分组增删改 + 访客可见开关 + 排序；卡片增删改 + 排序

## 构建

```bash
cd web-src
npm install
npm run build   # 产物输出到 ../web/（index.html + assets/ + logo.png + favicon.ico）
```

构建前确保 `web-src/public/` 下有 `logo.png` / `favicon.ico`
（Vite 会自动复制到 `web/`；`emptyOutDir` 会清空 web/ 后重建）。

## 依赖（仅 2 个运行时依赖）

- vue 3.4, vue-router 4.2 — 无 UI 库，手写 CSS，JS bundle ~112KB（gzip ~43KB）

## 后端配合要求（service/，需主 agent 处理）

1. SPA fallback：后端目前只在精确 `/` 提供 index.html，
   `/login` `/admin` `/auth/callback` 直接访问会 404。
   建议加 `router.NoRoute()` 返回 `web/index.html`。
   否则 R通行证 OAuth 回调（redirect_uri=.../auth/callback）无法工作。
2. `/logo.png` 目前没有静态路由（只有 /favicon.ico 有），
   需加 `router.StaticFile("/logo.png", webPath+"/logo.png")`，
   否则顶栏 logo 404（前端有降级：隐藏图片只显示文字）。
