<template>
  <main class="main">
    <div class="toolbar">
      <button class="hbtn" @click="$router.push('/admin')">← 返回管理</button>
      <div class="spacer"></div>
      <button class="hbtn" @click="load" :disabled="loading">↻ 刷新</button>
    </div>

    <div v-if="error" class="err">{{ error }}</div>

    <div v-if="loading" class="state">
      <div class="spinner"></div>
      <div>加载容器列表…</div>
    </div>
    <div v-else-if="!containers.length" class="state">
      <div class="big">没有找到容器</div>
      <div>Docker 可能未运行，或后端接口尚未就绪</div>
    </div>
    <div v-else class="docker-grid">
      <div v-for="c in containers" :key="c.id" class="dcard">
        <div class="dcard-head">
          <div class="dcard-name">{{ c.name }}</div>
          <span class="status" :class="c.state">{{ statusText(c.state) }}</span>
        </div>
        <div class="dcard-img">{{ c.image }}</div>
        <div class="dcard-ports">{{ c.ports || '无端口映射' }}</div>
        <div class="dcard-acts">
          <button class="mini" v-if="c.state !== 'running'" :disabled="busy" @click="operate(c, 'start')">启动</button>
          <button class="mini" v-if="c.state === 'running'" :disabled="busy" @click="operate(c, 'stop')">停止</button>
          <button class="mini" :disabled="busy" @click="operate(c, 'restart')">重启</button>
          <button class="mini" @click="openLogs(c)">日志</button>
          <button class="mini" v-if="c.state === 'running'" @click="openStats(c)">状态</button>
        </div>
      </div>
    </div>

    <!-- 日志弹窗 -->
    <div v-if="logTarget" class="modal-mask" @click.self="logTarget = null">
      <div class="modal wide">
        <h2>{{ logTarget.name }} <span class="tag">最近 200 行</span></h2>
        <div v-if="logLoading" class="state" style="padding:30px"><div class="spinner"></div></div>
        <pre v-else class="logs">{{ logs || '(无日志输出)' }}</pre>
        <div class="foot">
          <button class="btn ghost" @click="logTarget = null">关闭</button>
          <button class="btn" @click="loadLogs(logTarget)">刷新</button>
        </div>
      </div>
    </div>

    <!-- 实时状态弹窗（OPT-014） -->
    <div v-if="statTarget" class="modal-mask" @click.self="closeStats">
      <div class="modal">
        <h2>{{ statTarget.name }} <span class="tag">实时状态</span></h2>
        <div v-if="statLoading" class="state" style="padding:30px"><div class="spinner"></div></div>
        <div v-else-if="stats" class="stats">
          <div class="srow"><span>CPU</span><b>{{ stats.cpuPercent.toFixed(1) }}%</b></div>
          <div class="bar"><i :style="{ width: Math.min(100, stats.cpuPercent) + '%' }"></i></div>
          <div class="srow"><span>内存</span><b>{{ stats.memUsageMB.toFixed(0) }} / {{ stats.memLimitMB.toFixed(0) }} MB（{{ stats.memPercent.toFixed(1) }}%）</b></div>
          <div class="bar"><i :style="{ width: Math.min(100, stats.memPercent) + '%' }"></i></div>
          <div class="srow"><span>网络接收</span><b>{{ stats.netRxMB.toFixed(1) }} MB</b></div>
          <div class="srow"><span>网络发送</span><b>{{ stats.netTxMB.toFixed(1) }} MB</b></div>
          <div class="snote">每 3 秒自动刷新</div>
        </div>
        <div v-else class="state">获取状态失败</div>
        <div class="foot">
          <button class="btn ghost" @click="closeStats">关闭</button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { api } from '../api.js'

const containers = ref([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const logTarget = ref(null)
const logs = ref('')
const logLoading = ref(false)
// OPT-014：实时状态
const statTarget = ref(null)
const stats = ref(null)
const statLoading = ref(false)
let statTimer = null

const stateText = {
  running: '运行中',
  exited: '已停止',
  created: '已创建',
  paused: '已暂停',
  restarting: '重启中',
  dead: '异常'
}
function statusText(s) {
  return stateText[s] || s || '未知'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const d = await api('/docker/containers', {})
    containers.value = Array.isArray(d) ? d : []
  } catch (e) {
    error.value = e.message === '服务器响应异常'
      ? 'Docker 管理后端接口尚未实现，请等待后端 agent 对齐 API_DOCKER.md'
      : e.message
  } finally {
    loading.value = false
  }
}

const actionText = { start: '启动', stop: '停止', restart: '重启' }
async function operate(c, action) {
  if (!confirm(`确定要${actionText[action]}容器「${c.name}」吗？`)) return
  busy.value = true
  try {
    await api('/docker/container/operate', { id: c.id, action })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

function openLogs(c) {
  logTarget.value = c
  logs.value = ''
  loadLogs(c)
}

async function loadLogs(c) {
  logLoading.value = true
  try {
    const d = await api('/docker/container/logs', { id: c.id, tail: 200 })
    logs.value = (d && d.logs) || ''
  } catch (e) {
    logs.value = '获取日志失败：' + e.message
  } finally {
    logLoading.value = false
  }
}

onMounted(load)

onUnmounted(closeStats)

// OPT-014：容器实时状态（3秒轮询）
async function openStats(c) {
  statTarget.value = c
  stats.value = null
  await loadStats(c)
  statTimer = setInterval(() => loadStats(c), 3000)
}
async function loadStats(c) {
  statLoading.value = !stats.value
  try {
    stats.value = await api('/docker/container/stats', { id: c.id })
  } catch (e) {
    if (!stats.value) stats.value = null
  } finally {
    statLoading.value = false
  }
}
function closeStats() {
  statTarget.value = null
  stats.value = null
  if (statTimer) { clearInterval(statTimer); statTimer = null }
}
</script>
