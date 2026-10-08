package mcp

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/mcp"

	"github.com/gin-gonic/gin"
)

// R导航内置MCP路由
func Init(routerGroup *gin.RouterGroup) {
	mcpApi := api_v1.ApiGroupApp.ApiMcp
	r := routerGroup.Group("mcp", mcp.McpAuthInterceptor)
	{
		r.POST("/listGroups", mcpApi.ListGroups)
		r.POST("/listCards", mcpApi.ListCards)
		r.POST("/addGroup", mcpApi.AddGroup)
		r.POST("/addCard", mcpApi.AddCard)
		r.POST("/updateCard", mcpApi.UpdateCard)
		r.POST("/deleteCard", mcpApi.DeleteCard)
	}
}

// InitServer 注册标准 MCP Server（JSON-RPC 2.0，Streamable HTTP）
// 端点：POST /mcp，与 REST 接口（/api/mcp/*）并存
func InitServer(e *gin.Engine) {
	e.POST("/mcp", mcp.McpAuthInterceptor, api_v1.ApiGroupApp.ApiMcp.JSONRPC)
}
