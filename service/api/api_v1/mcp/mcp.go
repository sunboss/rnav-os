package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/global"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
)

// 内置 MCP 管理接口 (R导航原生)
// 认证: Header Authorization: Bearer <mcp_token>
// Token 在 conf.ini [mcp] token 中配置，留空则首次启动自动生成
// OPT-014：Handler 已瘦身为 core 层薄封装；另见 server.go（标准 JSON-RPC MCP Server）

type McpApi struct {
}

func getMcpToken() string {
	token := global.Config.GetValueString("mcp", "token") // 尝试从系统设置读取
	if token == "" {
		if v, err := global.SystemSetting.GetValueString("mcp_token"); err == nil && v != "" {
			return v
		}
		// 自动生成并保存到系统设置
		b := make([]byte, 24)
		_, _ = rand.Read(b)
		token = hex.EncodeToString(b)
		_ = global.SystemSetting.Set("mcp_token", token)
		global.Logger.Info("MCP token 已自动生成并保存到系统设置")
	}
	return token
}

// MCP Token 认证中间件
func McpAuthInterceptor(c *gin.Context) {
	if global.Config.GetValueStringOrDefault("mcp", "enabled") != "true" {
		apiReturn.Error(c, "MCP接口未启用")
		c.Abort()
		return
	}
	auth := c.GetHeader("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" || token != getMcpToken() {
		apiReturn.ErrorCode(c, 1001, "MCP token无效", nil)
		c.Abort()
		return
	}
	c.Next()
}

// MCP 操作默认使用的用户ID（取第一个管理员）
func mcpUserId() uint {
	var user models.User
	if err := global.Db.Where("role=1").Order("id").First(&user).Error; err != nil {
		if err2 := global.Db.Order("id").First(&user).Error; err2 != nil {
			return 0
		}
	}
	return user.ID
}

// @Summary MCP: 获取分组列表
func (m *McpApi) ListGroups(c *gin.Context) {
	result, err := coreListGroups()
	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, result)
}

// @Summary MCP: 获取卡片列表
func (m *McpApi) ListCards(c *gin.Context) {
	result, err := coreListCards(c.Query("group"))
	if err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, result)
}

type addGroupReq struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	PublicVisitHide int    `json:"publicVisitHide"`
}

// @Summary MCP: 新增分组
func (m *McpApi) AddGroup(c *gin.Context) {
	req := addGroupReq{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "title必填")
		return
	}
	result, err := coreAddGroup(req.Title, req.Description, req.PublicVisitHide)
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, result)
}

type addCardReq struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	Group       string `json:"group"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	OpenMethod  int    `json:"openMethod"`
}

// @Summary MCP: 新增卡片（分组不存在自动创建）
func (m *McpApi) AddCard(c *gin.Context) {
	req := addCardReq{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "title和url必填")
		return
	}
	result, err := coreAddCard(coreAddCardParams{
		Title: req.Title, Url: req.Url, Group: req.Group,
		Description: req.Description, Icon: req.Icon, OpenMethod: req.OpenMethod,
	})
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, result)
}

type updateCardReq struct {
	CardId      uint   `json:"card_id"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	OpenMethod  *int   `json:"openMethod"`
	Group       string `json:"group"`
}

// @Summary MCP: 更新卡片
func (m *McpApi) UpdateCard(c *gin.Context) {
	req := updateCardReq{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "card_id必填")
		return
	}
	result, err := coreUpdateCard(coreUpdateCardParams{
		CardId: req.CardId, Title: req.Title, Url: req.Url,
		Description: req.Description, Icon: req.Icon,
		OpenMethod: req.OpenMethod, Group: req.Group,
	})
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, result)
}

// @Summary MCP: 删除卡片
func (m *McpApi) DeleteCard(c *gin.Context) {
	var req struct {
		CardId uint `json:"card_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, "card_id必填")
		return
	}
	if err := coreDeleteCard(req.CardId); err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.Success(c)
}
