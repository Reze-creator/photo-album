import { reactive } from 'vue'

const storageKey = 'photo-album-session'

function readSession() {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey))
    if (typeof saved?.token === 'string' && saved.user?.id) return saved
  } catch {
    // 缓存损坏或浏览器禁用存储时，仍然可以使用当前页面。
  }
  return { token: '', user: null }
}

export const session = reactive(readSession())

export function setSession(data) {
  session.token = data?.token || ''
  session.user = data?.user || null
  try {
    if (data) localStorage.setItem(storageKey, JSON.stringify(data))
    else localStorage.removeItem(storageKey)
  } catch {
    // 存储被禁用时保留内存中的登录状态。
  }
}
