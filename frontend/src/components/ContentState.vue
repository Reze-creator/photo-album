<script setup>
import { FolderOpen, CircleAlert, LoaderCircle } from '@lucide/vue'

const props = defineProps({ loading: Boolean, error: Object, title: String, detail: String })
const emit = defineEmits(['retry'])
</script>

<template>
  <div class="state-panel" :role="props.error ? 'alert' : 'status'">
    <span class="state-icon" aria-hidden="true">
      <LoaderCircle v-if="props.loading" class="spin" :size="26" />
      <CircleAlert v-else-if="props.error" :size="26" />
      <FolderOpen v-else :size="26" />
    </span>
    <h2>{{ props.loading ? '正在加载' : props.error ? '无法加载内容' : props.title }}</h2>
    <p>{{ props.error?.message || props.detail }}</p>
    <slot />
    <button v-if="props.error && ![401, 403, 404].includes(props.error.status)" type="button" class="button secondary" @click="emit('retry')">重试</button>
  </div>
</template>
