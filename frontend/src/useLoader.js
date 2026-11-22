import { onScopeDispose, ref } from 'vue'

// 切换页面或翻页时取消旧请求，避免较慢的响应覆盖最新结果。
export function useLoader(fetchData) {
  const data = ref(null)
  const loading = ref(true)
  const error = ref(null)
  let controller

  async function load(...args) {
    controller?.abort()
    const current = new AbortController()
    controller = current
    data.value = null
    error.value = null
    loading.value = true
    try {
      const result = await fetchData(...args, current.signal)
      if (!current.signal.aborted) data.value = result
    } catch (reason) {
      if (!current.signal.aborted) error.value = reason
    } finally {
      if (!current.signal.aborted) loading.value = false
    }
  }
  onScopeDispose(() => controller?.abort())
  return { data, loading, error, load }
}
