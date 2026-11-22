<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Aperture, ArrowRight, ImagePlus } from '@lucide/vue'
import { api, checkImage } from '../api'
import { setSession } from '../session'

const props = defineProps({ register: Boolean })
const route = useRoute()
const router = useRouter()
const username = ref('')
const password = ref('')
const avatar = ref(null)
const error = ref('')
const busy = ref(false)
const notice = computed(() => !props.register && route.query.registered === '1')

watch(() => props.register, () => {
  username.value = ''
  password.value = ''
  avatar.value = null
  error.value = ''
})

function chooseAvatar(event) {
  avatar.value = event.target.files[0] || null
  error.value = avatar.value ? checkImage(avatar.value, 2) : ''
}

async function submit() {
  if (busy.value) return
  error.value = props.register ? checkImage(avatar.value, 2) : ''
  if (error.value) return
  if (props.register && new TextEncoder().encode(password.value).length > 72) {
    error.value = '密码不能超过 72 字节'
    return
  }
  busy.value = true
  try {
    if (props.register) {
      const body = new FormData()
      body.append('username', username.value.trim())
      body.append('password', password.value)
      body.append('avatar', avatar.value)
      await api.register(body)
      await router.push({ name: 'login', query: { registered: '1', next: route.query.next } })
    } else {
      const result = await api.login({ username: username.value.trim(), password: password.value })
      setSession(result)
      // 仅允许回到站内的受保护页面，避免把查询参数当作外部跳转地址。
      const next = route.query.next
      const allowed = typeof next === 'string' && /^\/(my-albums|albums\/\d+)(?:[?#].*)?$/.test(next)
      await router.replace(allowed ? next : { name: 'mine' })
    }
  } catch (reason) {
    error.value = reason.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="auth-card">
    <span class="brand-icon auth-icon"><Aperture :size="25" /></span>
    <h1>{{ register ? '创建账号' : '登录留影' }}</h1>
    <p v-if="notice" role="status" class="success-message">注册成功，请登录。</p>
    <form class="form-stack" @submit.prevent="submit">
      <label>用户名<input v-model="username" required minlength="2" maxlength="30" autocomplete="username" /></label>
      <label>密码<input v-model="password" required :minlength="register ? 8 : undefined" type="password"
        :autocomplete="register ? 'new-password' : 'current-password'" /></label>
      <label v-if="register">头像
        <span class="file-picker"><ImagePlus :size="19" />{{ avatar?.name || '选择头像' }}
          <input type="file" required accept=".jpg,.jpeg,.png,.gif" @change="chooseAvatar" />
        </span>
        <small>JPG、PNG 或 GIF，最大 2 MB</small>
      </label>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <button class="button primary" :disabled="busy">{{ busy ? '正在提交…' : register ? '注册' : '登录' }}<ArrowRight :size="18" /></button>
    </form>
    <p class="auth-switch">
      {{ register ? '已有账号？' : '还没有账号？' }}
      <RouterLink :to="{ name: register ? 'login' : 'register', query: { next: route.query.next } }">{{ register ? '登录' : '去注册' }}</RouterLink>
    </p>
  </section>
</template>
