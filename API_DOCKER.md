# Docker 管理 API 契约（R导航自研版）

> 状态：**后端已实现**（2026-09-29，OPT-009），前端已按此契约实现（`web-src/src/views/Docker.vue`）。
> 所有接口均为 `POST + JSON`，返回 `{code, msg?, data?}`；`code: 0` 表示成功。
> 必须要求管理员登录（token 校验 + 管理员角色校验），普通登录用户和访客无权访问。
> 后端实现：`service/api/api_v1/panel/docker.go`（标准库直调 unix socket Engine API v1.43，无 docker client 依赖）；路由 `service/router/panel/docker.go`（LoginInterceptor + AdminInterceptor 双拦截）；配置 `[docker] socket_path` 默认 `/var/run/docker.sock`。

## 1. 容器列表

`POST /api/docker/containers`，body `{}`

成功返回：

```json
{
  "code": 0,
  "data": [
    {
      "id": "a1b2c3d4e5f6",
      "name": "sun-panel",
      "image": "hslr/sun-panel:2.0.0-dev-latest",
      "status": "Up 3 days",
      "state": "running",
      "ports": "0.0.0.0:3002->3002/tcp"
    }
  ]
}
```

字段说明：
- `id`：容器 ID（可短 ID，前端原样回传）
- `name`：容器名（去掉前导 `/`）
- `image`：镜像名
- `status`：docker 原生状态文本（展示用）
- `state`：`running` | `exited` | `created` | `paused` | `restarting` | `dead`（前端据此渲染状态徽标和按钮）
- `ports`：端口映射展示字符串，无映射传空字符串

## 2. 容器操作

`POST /api/docker/container/operate`，body：

```json
{ "id": "a1b2c3d4e5f6", "action": "start" }
```

- `action` 仅允许 `start` | `stop` | `restart`，其他值返回参数错误
- 成功返回 `{code: 0, msg: "操作成功"}`

安全约束（必须遵守）：
- **不提供**删除容器、exec 终端、创建容器、镜像管理
- 后端直连本机 docker.sock 即可，前端不接触 socket

## 3. 容器日志

`POST /api/docker/container/logs`，body：

```json
{ "id": "a1b2c3d4e5f6", "tail": 200 }
```

成功返回：

```json
{ "code": 0, "data": { "logs": "...最近 200 行原始日志文本..." } }
```

- `tail` 默认 200，最大不超过 1000
- 返回 stdout+stderr 合并文本即可

## 前端已做
- `/admin/docker` 路由（登录守卫），管理页有入口按钮
- 卡片风格列表：容器名 / 镜像 / 状态徽标（运行中绿 / 已停止灰 / 暂停黄 / 异常红）/ 端口
- 启动 / 停止 / 重启（二次确认），日志弹窗（等宽字体，最近 200 行）
- 后端未实现时显示友好提示，不白屏
