<script setup>
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from '@lucide/vue'

const props = defineProps({ page: { type: Object, required: true } })
const emit = defineEmits(['change'])
const pageCount = computed(() => Math.max(1, Math.ceil(props.page.total / props.page.page_size)))
</script>

<template>
  <nav v-if="pageCount > 1" class="pager" aria-label="分页">
    <button type="button" class="button-icon" :disabled="page.page === 1" @click="emit('change', page.page - 1)" aria-label="上一页"><ChevronLeft :size="19" /></button>
    <span>第 {{ page.page }} / {{ pageCount }} 页</span>
    <button type="button" class="button-icon" :disabled="page.page >= pageCount" @click="emit('change', page.page + 1)" aria-label="下一页"><ChevronRight :size="19" /></button>
  </nav>
</template>
