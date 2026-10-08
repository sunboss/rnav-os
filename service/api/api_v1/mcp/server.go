package mcp

import (
	"encoding/json"
	"fmt"

	"sun-panel/api/api_v1/panel"

	"github.com/gin-gonic/gin"
)

// OPT-014：标准 MCP Server（JSON-RPC 2.0，Streamable HTTP）
// 端点：POST /mcp（Bearer Token 认证，复用 McpAuthInterceptor）
// 原有 REST 接口（/api/mcp/*）保持不变，mcp-gateway 继续可用。
// 支持方法：initialize / notifications/initialized / tools/list / tools/call

const mcpProtocolVersion = "2024-11-05"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

// MCP 工具定义
type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

var mcpTools = []mcpTool{
	{
		Name: "rnav_list_groups", Description: "获取导航分组列表",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	},
	{
		Name: "rnav_list_cards", Description: "获取导航卡片列表，可按分组标题过滤",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
			"group": map[string]interface{}{"type": "string", "description": "分组标题，为空返回全部"},
		}},
	},
	{
		Name: "rnav_add_group", Description: "新增导航分组（已存在则直接返回）",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"title"}, "properties": map[string]interface{}{
			"title":           map[string]interface{}{"type": "string"},
			"description":     map[string]interface{}{"type": "string"},
			"publicVisitHide": map[string]interface{}{"type": "integer", "description": "1=访客隐藏"},
		}},
	},
	{
		Name: "rnav_add_card", Description: "新增导航卡片，分组不存在时自动创建",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"title", "url"}, "properties": map[string]interface{}{
			"title":       map[string]interface{}{"type": "string"},
			"url":         map[string]interface{}{"type": "string"},
			"group":       map[string]interface{}{"type": "string", "description": "分组标题，默认APP"},
			"description": map[string]interface{}{"type": "string"},
			"icon":        map[string]interface{}{"type": "string", "description": "iconify 图标名，如 material-symbols:link"},
			"openMethod":  map[string]interface{}{"type": "integer", "description": "打开方式，默认2=新标签页"},
		}},
	},
	{
		Name: "rnav_update_card", Description: "更新导航卡片（只传要改的字段）",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"card_id"}, "properties": map[string]interface{}{
			"card_id":     map[string]interface{}{"type": "integer"},
			"title":       map[string]interface{}{"type": "string"},
			"url":         map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
			"icon":        map[string]interface{}{"type": "string"},
			"openMethod":  map[string]interface{}{"type": "integer"},
			"group":       map[string]interface{}{"type": "string", "description": "移动到指定分组标题"},
		}},
	},
	{
		Name: "rnav_delete_card", Description: "删除导航卡片",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"card_id"}, "properties": map[string]interface{}{
			"card_id": map[string]interface{}{"type": "integer"},
		}},
	},
	{
		Name: "rnav_docker_containers", Description: "获取 Docker 容器列表",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	},
	{
		Name: "rnav_docker_operate", Description: "容器启停/重启（仅允许 start/stop/restart）",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"id", "action"}, "properties": map[string]interface{}{
			"id":     map[string]interface{}{"type": "string", "description": "容器ID（12-64位hex）"},
			"action": map[string]interface{}{"type": "string", "enum": []string{"start", "stop", "restart"}},
		}},
	},
	{
		Name: "rnav_docker_logs", Description: "获取容器日志",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"id"}, "properties": map[string]interface{}{
			"id":   map[string]interface{}{"type": "string"},
			"tail": map[string]interface{}{"type": "integer", "description": "最后N行，默认200，最大1000"},
		}},
	},
	{
		Name: "rnav_docker_stats", Description: "获取容器实时状态（CPU/内存/网络）",
		InputSchema: map[string]interface{}{"type": "object", "required": []string{"id"}, "properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"},
		}},
	},
}

// JSONRPC 标准 MCP 入口：POST /mcp
func (m *McpApi) JSONRPC(c *gin.Context) {
	var req jsonRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", Error: &jsonRPCError{Code: -32700, Message: "Parse error"}})
		return
	}
	if req.JSONRPC != "2.0" {
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32600, Message: "Invalid Request: jsonrpc must be 2.0"}})
		return
	}

	switch req.Method {
	case "initialize":
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "rnav-mcp", "version": "1.0.0"},
		}})
		return
	case "notifications/initialized":
		// 通知无 ID，无需响应体
		c.Status(202)
		return
	case "tools/list":
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{"tools": mcpTools}})
		return
	case "tools/call":
		result, callErr := mcpCallTool(req.Params)
		if callErr != nil {
			c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32602, Message: callErr.Error()}})
			return
		}
		text, _ := json.Marshal(result)
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": string(text)}},
		}})
		return
	default:
		c.JSON(200, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "Method not found: " + req.Method}})
		return
	}
}

type mcpCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

func argStr(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func argInt(args map[string]interface{}, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func argUint(args map[string]interface{}, key string) uint {
	return uint(argInt(args, key))
}

// mcpCallTool 执行工具调用，返回可 JSON 序列化的结果
func mcpCallTool(raw json.RawMessage) (interface{}, error) {
	var p mcpCallParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid tools/call params")
	}
	args := p.Arguments
	if args == nil {
		args = map[string]interface{}{}
	}
	switch p.Name {
	case "rnav_list_groups":
		return coreListGroups()
	case "rnav_list_cards":
		return coreListCards(argStr(args, "group"))
	case "rnav_add_group":
		return coreAddGroup(argStr(args, "title"), argStr(args, "description"), argInt(args, "publicVisitHide"))
	case "rnav_add_card":
		return coreAddCard(coreAddCardParams{
			Title: argStr(args, "title"), Url: argStr(args, "url"), Group: argStr(args, "group"),
			Description: argStr(args, "description"), Icon: argStr(args, "icon"), OpenMethod: argInt(args, "openMethod"),
		})
	case "rnav_update_card":
		var openMethod *int
		if _, ok := args["openMethod"]; ok {
			v := argInt(args, "openMethod")
			openMethod = &v
		}
		return coreUpdateCard(coreUpdateCardParams{
			CardId: argUint(args, "card_id"), Title: argStr(args, "title"), Url: argStr(args, "url"),
			Description: argStr(args, "description"), Icon: argStr(args, "icon"),
			OpenMethod: openMethod, Group: argStr(args, "group"),
		})
	case "rnav_delete_card":
		if err := coreDeleteCard(argUint(args, "card_id")); err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true}, nil
	case "rnav_docker_containers":
		return panel.CoreDockerContainers()
	case "rnav_docker_operate":
		if err := panel.CoreDockerOperate(argStr(args, "id"), argStr(args, "action")); err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true}, nil
	case "rnav_docker_logs":
		logs, err := panel.CoreDockerLogs(argStr(args, "id"), argInt(args, "tail"))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"logs": logs}, nil
	case "rnav_docker_stats":
		return panel.CoreDockerStats(argStr(args, "id"))
	default:
		return nil, fmt.Errorf("unknown tool: %s", p.Name)
	}
}
