// R导航 API 封装
const TOKEN_KEY = 'rnav_token'
const USER_KEY = 'rnav_user'

export const getToken = () => {
  try { return localStorage.getItem(TOKEN_KEY) } catch { return null }
}
export const isLoggedIn = () => !!getToken()
// R导航：隐藏分组（内网）仅管理员可见，管理后台与 Docker 页仅管理员可进
export const isAdmin = () => getUser()?.role === 1
export const getUser = () => {
  try { return JSON.parse(localStorage.getItem(USER_KEY) || 'null') } catch { return null }
}
export function setSession(token, user) {
  try {
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(USER_KEY, JSON.stringify(user || {}))
  } catch {}
}
export function clearSession() {
  try {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
  } catch {}
}

function handleResult(j, redirectOnExpire = true) {
  if (j.code === 1001) {
    clearSession()
    if (redirectOnExpire && location.pathname !== '/login') location.href = '/login'
    throw new Error('登录已过期，请重新登录')
  }
  if (j.code !== 0) throw new Error(j.msg || '请求失败')
  return j.data
}

async function parseJson(res) {
  try { return await res.json() }
  catch { throw new Error('服务器响应异常') }
}

// POST /api/xxx
export async function api(path, body, opts = {}) {
  const headers = { 'Content-Type': 'application/json' }
  const t = getToken()
  if (opts.auth !== false && t) headers['token'] = t
  let res
  try {
    res = await fetch('/api' + path, {
      method: 'POST',
      headers,
      body: JSON.stringify(body ?? {})
    })
  } catch {
    throw new Error('网络错误，请检查连接')
  }
  return handleResult(await parseJson(res), opts.redirectOnExpire)
}

// GET /api/xxx
export async function apiGet(path, opts = {}) {
  const headers = {}
  const t = getToken()
  if (opts.auth !== false && t) headers['token'] = t
  let res
  try {
    res = await fetch('/api' + path, { headers })
  } catch {
    throw new Error('网络错误，请检查连接')
  }
  return handleResult(await parseJson(res), opts.redirectOnExpire)
}

// iconify 图标转直链 SVG（免 JS 依赖）
export function iconifyUrl(name) {
  return 'https://api.iconify.design/' + encodeURIComponent(name).replace(/%3A/g, ':') + '.svg'
}
