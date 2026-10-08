<template>
  <div class="state" style="margin-top:80px">
    <div class="spinner"></div>
    <div class="big">R通行证登录中…</div>
    <div v-if="error" class="err" style="max-width:380px;margin:14px auto 0">{{ error }}</div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api, setSession } from '../api.js'

const router = useRouter()
const route = useRoute()
const error = ref('')

onMounted(async () => {
  const code = route.query.code
  const state = route.query.state
  if (!code || !state) {
    error.value = '缺少授权参数，请重新登录'
    setTimeout(() => router.replace('/login'), 2000)
    return
  }
  try {
    const d = await api('/rpass/callback', { code, state }, { auth: false, redirectOnExpire: false })
    if (!d || !d.token) throw new Error('登录失败，未获取到 token')
    setSession(d.token, d)
    router.replace('/')
  } catch (e) {
    error.value = e.message
    setTimeout(() => router.replace('/login'), 2500)
  }
})
</script>
