<template>
  <div class="login-wrap">
    <div class="login-card">
      <h1>R导航</h1>
      <div class="sub">登录后可管理卡片与分组</div>
      <div v-if="error" class="err">{{ error }}</div>
      <form @submit.prevent="doLogin">
        <div class="field">
          <label>账号</label>
          <input class="input" v-model="username" placeholder="用户名" autocomplete="username">
        </div>
        <div class="field">
          <label>密码</label>
          <input class="input" type="password" v-model="password" placeholder="密码" autocomplete="current-password">
        </div>
        <button class="btn" :disabled="busy">{{ busy ? '登录中…' : '登录' }}</button>
      </form>
      <div class="divider">或</div>
      <button class="btn ghost" :disabled="busy" @click="rpassLogin">🔑 R通行证登录</button>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api, apiGet, setSession, isLoggedIn } from '../api.js'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

if (isLoggedIn()) {
  router.replace(route.query.from || '/')
}

async function doLogin() {
  error.value = ''
  if (!username.value.trim() || !password.value) {
    error.value = '请输入账号和密码'
    return
  }
  busy.value = true
  try {
    const d = await api('/login', { username: username.value.trim(), password: password.value }, { auth: false, redirectOnExpire: false })
    if (!d || !d.token) throw new Error('登录失败，未获取到 token')
    setSession(d.token, d)
    router.replace(route.query.from || '/')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function rpassLogin() {
  error.value = ''
  busy.value = true
  try {
    const d = await apiGet('/rpass/auth-url', { auth: false, redirectOnExpire: false })
    if (!d || !d.authUrl) throw new Error('获取授权地址失败')
    location.href = d.authUrl
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}
</script>
