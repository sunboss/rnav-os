import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn, isAdmin } from './api.js'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import AuthCallback from './views/AuthCallback.vue'
import Admin from './views/Admin.vue'
import Docker from './views/Docker.vue'

const routes = [
  { path: '/', name: 'home', component: Home },
  { path: '/login', name: 'login', component: Login },
  { path: '/auth/callback', name: 'callback', component: AuthCallback },
  { path: '/admin', name: 'admin', component: Admin, meta: { needLogin: true } },
  { path: '/admin/docker', name: 'docker', component: Docker, meta: { needLogin: true } },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})

router.beforeEach((to) => {
  if (to.meta.needLogin && !isLoggedIn()) {
    return { path: '/login', query: { from: to.fullPath } }
  }
  // 管理后台仅管理员可进：普通登录用户直接回首页（后端同样会以 1005 拦截）
  if ((to.name === 'admin' || to.name === 'docker') && isLoggedIn() && !isAdmin()) {
    return { path: '/' }
  }
})

export default router
