# R导航 (rnav-os)

自研网址导航系统，简洁、快速、美观。

## 特性

- 🎨 彩带 R 流动美学，深浅双模式
- 🔍 书签搜索（支持拼音首字母、模糊匹配）
- 📱 响应式设计，手机自适应
- 🔐 R通行证统一登录
- 🐳 Docker 一键部署
- 📊 MCP Server 支持

## 快速开始

```bash
# 构建前端
cd web-src && npm run build

# 启动
docker compose -f docker-compose.rnav.yml up -d --build
```

## 技术栈

- 后端：Go (Gin)
- 前端：Vue 3 + Vite
- 数据库：SQLite

## 部署

生产环境：https://home.1234r.com

## License

MIT
