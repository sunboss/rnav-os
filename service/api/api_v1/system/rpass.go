package system

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/global"
	"sun-panel/lib/cmn"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// R通行证 (Casdoor) OAuth 登录
type RpassApi struct {
}

// OPT-012：复用 HTTP client（连接复用，避免每次回调新建连接池）
var rpassHTTPClient = &http.Client{Timeout: 15 * time.Second}

// OPT-012：回调请求体/参数上限
const (
	rpassMaxBodyBytes = 32 << 10  // 回调 JSON 最多 32KB
	rpassMaxRespBytes = 1 << 20   // 上游响应最多 1MB
	rpassMaxCodeLen   = 512
	rpassMaxStateLen  = 64
)

// 生成 PKCE code_verifier
func rpassGenVerifier() string {
	b := make([]byte, 48)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// PKCE S256 challenge
func rpassChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func rpassCfg(key string) string {
	return global.Config.GetValueStringOrDefault("rpass", key)
}

// @Summary 获取R通行证授权地址
// @Router /rpass/auth-url [get]
func (r *RpassApi) AuthUrl(c *gin.Context) {
	if rpassCfg("enabled") != "true" {
		apiReturn.Error(c, "R通行证登录未启用")
		return
	}
	endpoint := strings.TrimRight(rpassCfg("endpoint"), "/")
	clientId := rpassCfg("client_id")
	redirectUri := rpassCfg("redirect_uri")

	verifier := rpassGenVerifier()
	challenge := rpassChallenge(verifier)
	state := uuid.NewString()

	// verifier 存入短期缓存(10分钟)，state 作为 key
	global.CUserToken.Set(state, verifier, 10*time.Minute)

	authUrl := endpoint + "/login/oauth/authorize" +
		"?client_id=" + url.QueryEscape(clientId) +
		"&response_type=code" +
		"&redirect_uri=" + url.QueryEscape(redirectUri) +
		"&scope=" + url.QueryEscape("read") +
		"&state=" + url.QueryEscape(state) +
		"&code_challenge=" + url.QueryEscape(challenge) +
		"&code_challenge_method=S256"

	apiReturn.SuccessData(c, gin.H{"authUrl": authUrl, "state": state})
}

type rpassCallbackReq struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// @Summary R通行证回调，用code换取本地token
// @Router /rpass/callback [post]
func (r *RpassApi) Callback(c *gin.Context) {
	// OPT-012：限制请求体大小，防超大 JSON 攻击
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, rpassMaxBodyBytes)
	param := rpassCallbackReq{}
	if err := c.ShouldBindJSON(&param); err != nil || param.Code == "" || param.State == "" {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}
	// OPT-012：参数长度上限，避免超长 key 打缓存/打上游
	if len(param.Code) > rpassMaxCodeLen || len(param.State) > rpassMaxStateLen {
		apiReturn.ErrorParamFomat(c, "参数错误")
		return
	}

	// 取回 PKCE verifier（单次使用，10分钟过期）
	verifierRaw, ok := global.CUserToken.Get(param.State)
	global.CUserToken.Delete(param.State)
	if !ok || verifierRaw == "" {
		apiReturn.ErrorParamFomat(c, "state已过期，请重新登录")
		return
	}
	verifier := verifierRaw

	endpoint := strings.TrimRight(rpassCfg("endpoint"), "/")
	clientId := rpassCfg("client_id")
	clientSecret := rpassCfg("client_secret")
	redirectUri := rpassCfg("redirect_uri")

	// 1. code 换 access_token
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientId)
	form.Set("client_secret", clientSecret)
	form.Set("code", param.Code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectUri)

	resp, err := rpassHTTPClient.PostForm(endpoint+"/api/login/oauth/access_token", form)
	if err != nil {
		// OPT-012：内部错误只记日志，不回传给客户端
		global.Logger.Warnf("rpass换取token失败: %v", err)
		apiReturn.Error(c, "换取token失败，请稍后重试")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		global.Logger.Warnf("rpass换取token非200: %d", resp.StatusCode)
		apiReturn.Error(c, "换取token失败，请稍后重试")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, rpassMaxRespBytes))
	tokenResp := map[string]interface{}{}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		apiReturn.Error(c, "token响应解析失败")
		return
	}
	accessToken, _ := tokenResp["access_token"].(string)
	if accessToken == "" {
		apiReturn.Error(c, "未获取到access_token")
		return
	}

	// 2. 获取用户信息
	req, _ := http.NewRequest("GET", endpoint+"/api/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	userResp, err := rpassHTTPClient.Do(req)
	if err != nil {
		global.Logger.Warnf("rpass获取用户信息失败: %v", err)
		apiReturn.Error(c, "获取用户信息失败，请稍后重试")
		return
	}
	defer userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		global.Logger.Warnf("rpass获取用户信息非200: %d", userResp.StatusCode)
		apiReturn.Error(c, "获取用户信息失败，请稍后重试")
		return
	}
	userBody, _ := io.ReadAll(io.LimitReader(userResp.Body, rpassMaxRespBytes))
	userInfo := map[string]interface{}{}
	if err := json.Unmarshal(userBody, &userInfo); err != nil {
		apiReturn.Error(c, "用户信息解析失败")
		return
	}
	rpassId, _ := userInfo["id"].(string)
	if rpassId == "" {
		rpassId, _ = userInfo["sub"].(string)
	}
	if rpassId == "" {
		apiReturn.Error(c, "未获取到用户ID")
		return
	}
	displayName, _ := userInfo["name"].(string)
	if displayName == "" {
		displayName, _ = userInfo["preferred_username"].(string)
	}
	avatar, _ := userInfo["avatar"].(string)
	email, _ := userInfo["email"].(string)

	// 3. 查找或创建本地用户
	mUser := models.User{}
	info, err := mUser.GetUserInfoByRpassId(rpassId)
	if err == gorm.ErrRecordNotFound {
		// 首次登录，创建本地用户（普通用户角色）
		info = models.User{
			Username: "rpass_" + rpassId,
			Password: cmn.PasswordEncryption(uuid.NewString()),
			Name:     displayName,
			HeadImage: avatar,
			Status:   1,
			Role:     2,
			Mail:     email,
			RpassId:  rpassId,
		}
		if err := global.Db.Create(&info).Error; err != nil {
			global.Logger.Warnf("rpass创建用户失败: %v", err)
			apiReturn.Error(c, "创建用户失败，请稍后重试")
			return
		}
	} else if err != nil {
		global.Logger.Warnf("rpass查询用户失败: %v", err)
		apiReturn.Error(c, "查询用户失败，请稍后重试")
		return
	}
	if info.Status != 1 {
		apiReturn.ErrorByCode(c, 1004)
		return
	}

	// 4. 生成本地 token（复用原有登录逻辑）
	bToken := info.Token
	if bToken == "" {
		for {
			bToken = cmn.BuildRandCode(32, cmn.RAND_CODE_MODE2)
			if _, err := mUser.GetUserInfoByToken(bToken); err != nil {
				mUser.UpdateUserInfoByUserId(info.ID, map[string]interface{}{"token": bToken})
				break
			}
		}
		info.Token = bToken
	}
	info.Password = ""
	info.ReferralCode = ""

	cToken := uuid.NewString() + "-" + cmn.Md5(cmn.Md5("userId"+strconv.Itoa(int(info.ID))))
	global.CUserToken.SetDefault(cToken, bToken)

	c.Set("userInfo", info)
	info.Token = cToken
	apiReturn.SuccessData(c, info)
}

// 兼容：让前端可以用 GET 方式触发回调（保留扩展）
var _ = bytes.MinRead
