<script setup>
import { watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { Aperture, LogOut } from '@lucide/vue'
import { session, setSession } from './session'

const route = useRoute()
const router = useRouter()

watch(() => session.token, (token) => {
  if (!token && route.meta.login) router.replace({ name: 'login', query: { next: route.fullPath } })
})

async function logout() {
  await router.push({ name: 'public' })
  setSession(null)
}
</script>

<template>
  <header class="topbar">
    <div class="topbar-inner">
      <RouterLink to="/" class="brand" aria-label="留影首页">
        <span class="brand-icon"><Aperture :size="23" /></span>
        <span>留影</span>
      </RouterLink>
      <nav class="topnav" aria-label="主导航">
        <RouterLink to="/">公开广场</RouterLink>
        <RouterLink to="/my-albums">我的相册</RouterLink>
      </nav>
      <div class="account" v-if="session.user">
        <img class="avatar" :src="session.user.avatar_url" alt="" />
        <span class="account-name">{{ session.user.username }}</span>
        <button type="button" class="button-icon" @click="logout" aria-label="退出登录" title="退出登录"><LogOut :size="17" /></button>
      </div>
      <div class="account" v-else>
        <RouterLink class="login-link" to="/login">登录</RouterLink>
        <RouterLink class="button primary small" to="/register">注册</RouterLink>
      </div>
    </div>
  </header>
  <main class="site-main"><RouterView /></main>
</template>
