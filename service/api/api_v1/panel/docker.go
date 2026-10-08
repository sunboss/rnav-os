package panel

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/global"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// R导航 Docker 管理器 (OPT-009)
// 直接通过 unix socket 调用 Docker Engine API，不引入 docker client 大依赖。
// 安全边界：只开放 容器列表/启动/停止/重启/日志；默认禁止 删除容器/exec/创建容器。
// 全部接口仅管理员可访问（路由层叠加 LoginInterceptor + AdminInterceptor）。
type Docker struct {
}

const dockerAPIVersion = "v1.43"

// 容器ID校验：64位完整ID或12位以上短ID（hex），防止路径穿越/注入
var dockerIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{12,64}$`)

// dockerSocketPath 读取配置的 socket 路径
func dockerSocketPath() string {
	if p := global.Config.GetValueString("docker", "socket_path"); p != "" {
		return p
	}
	return "/var/run/docker.sock"
}

// newDockerClient 创建直连 unix socket 的 HTTP 客户端
// socket 不存在/不可用时返回干净错误，绝不崩溃
func newDockerClient() (*http.Client, error) {
	sock := dockerSocketPath()
	fi, err := os.Stat(sock)
	if err != nil {
		return nil, fmt.Errorf("Docker 管理器不可用：socket 文件不存在（%s），请确认已挂载 /var/run/docker.sock", sock)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("Docker 管理器不可用：%s 不是有效的 socket 文件", sock)
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := &net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, "unix", sock)
			},
		},
		Timeout: 20 * time.Second,
	}, nil
}

// dockerDo 执行一次 Engine API 调用
func dockerDo(client *http.Client, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://localhost/"+dockerAPIVersion+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

// validateDockerID 校验容器ID格式
func validateDockerID(id string) bool {
	return dockerIDPattern.MatchString(id)
}

// --- Engine API 返回结构 ---

type dockerPort struct {
	IP          string `json:"IP"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}

type dockerContainerJSON struct {
	Id     string       `json:"Id"`
	Names  []string     `json:"Names"`
	Image  string       `json:"Image"`
	State  string       `json:"State"`
	Status string       `json:"Status"`
	Ports  []dockerPort `json:"Ports"`
}

// OPT-014：容器实时 stats（Engine API /containers/{id}/stats?stream=false&one-shot=true）
type dockerStatsJSON struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// ContainerStats 对前端暴露的容器实时状态
type ContainerStats struct {
	CPUPercent float64 `json:"cpuPercent"` // CPU 使用率 %
	MemUsageMB float64 `json:"memUsageMB"` // 内存已用 MB
	MemLimitMB float64 `json:"memLimitMB"` // 内存上限 MB
	MemPercent float64 `json:"memPercent"` // 内存使用率 %
	NetRxMB    float64 `json:"netRxMB"`    // 累计接收 MB
	NetTxMB    float64 `json:"netTxMB"`    // 累计发送 MB
}

// ContainerInfo 对前端暴露的容器信息
type ContainerInfo struct {
	ID     string `json:"id"`     // 短ID（12位）
	Name   string `json:"name"`   // 容器名（去前缀/）
	Image  string `json:"image"`  // 镜像
	Status string `json:"status"` // 状态描述，如 "Up 2 hours"
	State  string `json:"state"`  // running / exited / paused / restarting / dead / created
	Ports  string `json:"ports"`  // 端口映射展示字符串，如 "0.0.0.0:3002->3002/tcp"，无映射为空字符串
}

// 容器列表
// POST /api/docker/containers
func (a *Docker) Containers(c *gin.Context) {
	out, err := CoreDockerContainers()
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, out)
}

// CoreDockerContainers 容器列表核心逻辑（供 REST 与 MCP Server 复用）
// OPT-014：从 Handler 中抽离
func CoreDockerContainers() ([]ContainerInfo, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, err
	}
	resp, err := dockerDo(client, http.MethodGet, "/containers/json?all=1", nil)
	if err != nil {
		return nil, fmt.Errorf("Docker Engine 调用失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker Engine 返回异常状态：%d", resp.StatusCode)
	}
	var list []dockerContainerJSON
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("Docker 容器列表解析失败：%w", err)
	}
	out := make([]ContainerInfo, 0, len(list))
	for _, dc := range list {
		name := ""
		if len(dc.Names) > 0 {
			name = strings.TrimPrefix(dc.Names[0], "/")
		}
		shortID := dc.Id
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		ports := make([]string, 0, len(dc.Ports))
		for _, p := range dc.Ports {
			ip := p.IP
			if ip == "" {
				ip = "0.0.0.0"
			}
			if p.PublicPort != 0 {
				ports = append(ports, fmt.Sprintf("%s:%d->%d/%s", ip, p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}
		out = append(out, ContainerInfo{
			ID:     shortID,
			Name:   name,
			Image:  dc.Image,
			Status: dc.Status,
			State:  dc.State,
			Ports:  strings.Join(ports, ", "),
		})
	}
	return out, nil
}

// 容器操作请求
type dockerOperateReq struct {
	ID     string `json:"id" binding:"required"`
	Action string `json:"action" binding:"required"` // start | stop | restart
}

// 容器启停/重启
// POST /api/docker/container/operate  body: {"id":"...","action":"start|stop|restart"}
// 安全：只允许这三个动作，不开放删除/exec/创建
func (a *Docker) Operate(c *gin.Context) {
	var req dockerOperateReq
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if err := CoreDockerOperate(req.ID, req.Action); err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.ApiReturn(c, 0, "操作成功", nil)
}

// CoreDockerOperate 容器启停/重启核心逻辑（供 REST 与 MCP Server 复用）
// OPT-014：从 Handler 中抽离
func CoreDockerOperate(id, action string) error {
	if !validateDockerID(id) {
		return fmt.Errorf("容器ID格式非法")
	}
	var endpoint string
	switch action {
	case "start":
		endpoint = "/containers/" + id + "/start"
	case "stop":
		endpoint = "/containers/" + id + "/stop"
	case "restart":
		endpoint = "/containers/" + id + "/restart"
	default:
		return fmt.Errorf("不支持的操作，仅允许 start/stop/restart")
	}
	client, err := newDockerClient()
	if err != nil {
		return err
	}
	resp, err := dockerDo(client, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("Docker Engine 调用失败：%w", err)
	}
	defer resp.Body.Close()
	// 204=成功执行；304=已经是目标状态（start 已运行的容器 / stop 已停止的容器）
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("容器操作失败（状态码 %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// 日志请求
type dockerLogsReq struct {
	ID   string `json:"id" binding:"required"`
	Tail int    `json:"tail"` // 取最后 N 行，默认200，最大5000
}

// 容器日志
// POST /api/docker/container/logs  body: {"id":"...","tail":200}
func (a *Docker) Logs(c *gin.Context) {
	var req dockerLogsReq
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	logs, err := CoreDockerLogs(req.ID, req.Tail)
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, gin.H{"logs": logs})
}

// CoreDockerLogs 容器日志核心逻辑（供 REST 与 MCP Server 复用）
// OPT-014：从 Handler 中抽离
func CoreDockerLogs(id string, tail int) (string, error) {
	if !validateDockerID(id) {
		return "", fmt.Errorf("容器ID格式非法")
	}
	if tail <= 0 {
		tail = 200
	}
	if tail > 1000 {
		tail = 1000
	}
	client, err := newDockerClient()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&tail=%d", id, tail)
	resp, err := dockerDo(client, http.MethodGet, path, nil)
	if err != nil {
		return "", fmt.Errorf("Docker Engine 调用失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取容器日志失败（状态码 %d）", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 最多10MB，防止内存爆炸
	if err != nil {
		return "", fmt.Errorf("读取容器日志失败：%w", err)
	}
	return demuxDockerLogs(raw), nil
}

// 容器实时状态
// POST /api/docker/container/stats  body: {"id":"..."}
// 返回 CPU/内存/网络累计值（one-shot，非流式）
func (a *Docker) Stats(c *gin.Context) {
	var req dockerLogsReq // 复用 {id, tail} 结构，只用 id
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	st, err := CoreDockerStats(req.ID)
	if err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, st)
}

// CoreDockerStats 容器实时状态核心逻辑（供 REST 与 MCP Server 复用）
// OPT-014：从 Handler 中抽离；CPU% 采用 Docker 官方算法
func CoreDockerStats(id string) (ContainerStats, error) {
	var zero ContainerStats
	if !validateDockerID(id) {
		return zero, fmt.Errorf("容器ID格式非法")
	}
	client, err := newDockerClient()
	if err != nil {
		return zero, err
	}
	resp, err := dockerDo(client, http.MethodGet, "/containers/"+id+"/stats?stream=false&one-shot=true", nil)
	if err != nil {
		return zero, fmt.Errorf("Docker Engine 调用失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("获取容器状态失败（状态码 %d）", resp.StatusCode)
	}
	var st dockerStatsJSON
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&st); err != nil {
		return zero, fmt.Errorf("容器状态解析失败：%w", err)
	}
	// CPU%：Docker 官方算法
	cpuPercent := 0.0
	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage) - float64(st.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(st.CPUStats.SystemCPUUsage) - float64(st.PreCPUStats.SystemCPUUsage)
	cpus := float64(st.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(st.CPUStats.CPUUsage.PercpuUsage))
	}
	if sysDelta > 0 && cpuDelta >= 0 && cpus > 0 {
		cpuPercent = cpuDelta / sysDelta * cpus * 100
	}
	memPercent := 0.0
	if st.MemoryStats.Limit > 0 {
		memPercent = float64(st.MemoryStats.Usage) / float64(st.MemoryStats.Limit) * 100
	}
	var rx, tx uint64
	for _, n := range st.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	const mb = 1024 * 1024
	return ContainerStats{
		CPUPercent: cpuPercent,
		MemUsageMB: float64(st.MemoryStats.Usage) / mb,
		MemLimitMB: float64(st.MemoryStats.Limit) / mb,
		MemPercent: memPercent,
		NetRxMB:    float64(rx) / mb,
		NetTxMB:    float64(tx) / mb,
	}, nil
}

// demuxDockerLogs 解析 Docker multiplexed stream（8字节帧头）
// 帧头：[0]=流类型(0stdin/1stdout/2stderr)，[1:4]=保留0，[4:8]=大端帧长度
// TTY 容器返回的是原始流，此时直接返回原文
func demuxDockerLogs(data []byte) string {
	if !looksMultiplexed(data) {
		return string(data)
	}
	var sb bytes.Buffer
	off := 0
	for off+8 <= len(data) {
		size := binary.BigEndian.Uint32(data[off+4 : off+8])
		off += 8
		if size == 0 {
			continue
		}
		if off+int(size) > len(data) {
			break
		}
		sb.Write(data[off : off+int(size)])
		off += int(size)
	}
	return sb.String()
}

// looksMultiplexed 只有当整个 buffer 能完整解析为帧时才认定为 multiplexed stream
func looksMultiplexed(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	off := 0
	for off+8 <= len(data) {
		if data[off] > 2 || data[off+1] != 0 || data[off+2] != 0 || data[off+3] != 0 {
			return false
		}
		size := binary.BigEndian.Uint32(data[off+4 : off+8])
		if size > 16<<20 { // 单帧超过16MB视为异常
			return false
		}
		off += 8
		if off+int(size) > len(data) {
			return false
		}
		off += int(size)
	}
	return off == len(data) && off > 0
}
