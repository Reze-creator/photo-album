<script setup>
import { ref, watch } from 'vue'
import { ImageOff, Images } from '@lucide/vue'
import { api } from '../api'

const props = defineProps({ path: String, protectedImage: Boolean, alt: { type: String, default: '' } })
const src = ref('')
const failed = ref(false)

watch(() => [props.path, props.protectedImage], async ([path, protectedImage], _, onCleanup) => {
  src.value = ''
  failed.value = false
  if (!path) return
  if (!protectedImage) {
    src.value = path
    return
  }
  // img 标签无法附加 Bearer Token。私有照片经授权下载，并及时回收对象 URL。
  const controller = new AbortController()
  let objectURL
  onCleanup(() => {
    controller.abort()
    if (objectURL) URL.revokeObjectURL(objectURL)
  })
  try {
    const blob = await api.image(path, controller.signal)
    if (controller.signal.aborted) return
    objectURL = URL.createObjectURL(blob)
    src.value = objectURL
  } catch {
    if (!controller.signal.aborted) failed.value = true
  }
}, { immediate: true })
</script>

<template>
  <img v-if="src && !failed" :src="src" :alt="alt" loading="lazy" @error="failed = true" />
  <div v-else class="photo-placeholder" :aria-label="failed ? '图片加载失败' : undefined">
    <ImageOff v-if="failed" :size="29" aria-hidden="true" />
    <Images v-else :size="29" aria-hidden="true" />
  </div>
</template>
