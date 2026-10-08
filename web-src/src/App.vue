<template>
  <header class="header">
    <router-link to="/" class="brand">
      <img src="/logo.png?v=20261004" alt="R导航" @error="logoOk = false" v-show="logoOk">
      <span class="ttl">导航</span>
    </router-link>
    <div class="spacer"></div>
    <nav>
      <router-link v-if="loggedIn && isAdmin()" to="/admin" class="hbtn">管理</router-link>
      <button class="hbtn icon" @click="toggleTheme" :title="dark ? '浅色模式' : '深色模式'">
        {{ dark ? '☀️' : '🌙' }}
      </button>
      <template v-if="loggedIn">
        <span class="tag">{{ userName }}</span>
        <button class="hbtn" @click="logout">退出</button>
      </template>
      <router-link v-else to="/login" class="hbtn primary">登录</router-link>
    </nav>
  </header>
  <router-view :key="$route.fullPath" />
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { getToken, getUser, clearSession, api, isAdmin } from './api.js'

const router = useRouter()
const route = useRoute()
const logoOk = ref(true)
const tokenTick = ref(0)

const loggedIn = computed(() => { tokenTick.value; return !!getToken() })
const userName = computed(() => { tokenTick.value; return getUser()?.name || getUser()?.username || '用户' })

const dark = ref(document.documentElement.getAttribute('data-theme') === 'dark')
function toggleTheme() {
  dark.value = !dark.value
  if (dark.value) {
    document.documentElement.setAttribute('data-theme', 'dark')
    try { localStorage.setItem('rnav_theme', 'dark') } catch {}
  } else {
    document.documentElement.removeAttribute('data-theme')
    try { localStorage.setItem('rnav_theme', 'light') } catch {}
  }
}

async function logout() {
  try { await api('/logout', {}, { redirectOnExpire: false }) } catch {}
  clearSession()
  tokenTick.value++
  router.push('/')
}

// 路由变化时刷新登录态
watch(() => route.fullPath, () => { tokenTick.value++ })
</script>
