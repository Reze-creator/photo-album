<script setup>
import { RouterLink } from 'vue-router'
import ContentState from '../components/ContentState.vue'
</script>

<template>
  <ContentState title="页面不存在" detail="该地址可能不正确。">
    <RouterLink class="button primary" to="/">返回公开广场</RouterLink>
  </ContentState>
</template>
