package api_v1

import (
	"sun-panel/api/api_v1/mcp"
	"sun-panel/api/api_v1/openness"
	"sun-panel/api/api_v1/panel"
	"sun-panel/api/api_v1/system"
)

type ApiGroup struct {
	ApiSystem system.ApiSystem // 系统功能api
	ApiOpen   openness.ApiPpenness
	ApiPanel  panel.ApiPanel
	ApiMcp    mcp.McpApi // R导航内置MCP接口
}

var ApiGroupApp = new(ApiGroup)
