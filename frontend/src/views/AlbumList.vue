<script setup>
import { reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight, Globe2, LockKeyhole, Plus } from '@lucide/vue'
import { api } from '../api'
import { useLoader } from '../useLoader'
import AlbumImage from '../components/AlbumImage.vue'
import ContentState from '../components/ContentState.vue'
import Pager from '../components/Pager.vue'

const props = defineProps({ mine: Boolean })
const page = ref(1)
const { data, loading, error, load } = useLoader((mine, currentPage, signal) => api.albums(mine, currentPage, signal))
const dialog = ref(null)
const draft = reactive({ name: '', description: '', is_public: false })
const saving = ref(false)
const formError = ref('')

function reload() { return load(props.mine, page.value) }

watch(() => props.mine, () => {
  page.value = 1
  reload()
}, { immediate: true })

function changePage(next) {
  page.value = next
  reload()
}

function openCreate() {
  Object.assign(draft, { name: '', description: '', is_public: false })
  formError.value = ''
  dialog.value.showModal()
}

async function createAlbum() {
  if (saving.value) return
  if (!draft.name.trim()) {
    formError.value = '请输入相册名称'
    return
  }
  saving.value = true
  formError.value = ''
  try {
    await api.createAlbum(draft)
    dialog.value?.close()
    changePage(1)
  } catch (reason) {
    formError.value = reason.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="page-heading">
    <div>
      <h1>{{ mine ? '我的相册' : '公开广场' }}</h1>
      <p>{{ mine ? '整理自己的照片，也可以将相册分享给访客。' : '浏览大家公开分享的相册。' }}</p>
    </div>
    <button v-if="mine" type="button" class="button primary" @click="openCreate"><Plus :size="19" />新建相册</button>
  </section>

  <ContentState v-if="loading || error" :loading="loading" :error="error" @retry="reload" />
  <ContentState v-else-if="!data?.items.length" :title="mine ? '还没有相册' : '暂时没有公开相册'"
    :detail="mine ? '创建第一本相册后，就可以上传照片。' : undefined" />
  <div v-else class="album-grid">
    <RouterLink v-for="album in data.items" :key="album.id" class="album-card" :to="{ name: 'album', params: { id: album.id } }">
      <div class="album-cover">
        <AlbumImage :path="album.cover_url" :protected-image="!album.is_public" />
        <span v-if="mine" class="privacy-chip">
          <Globe2 v-if="album.is_public" :size="14" /><LockKeyhole v-else :size="14" />
          {{ album.is_public ? '公开' : '私有' }}
        </span>
      </div>
      <div class="album-card-body">
        <div class="card-title-row"><h2>{{ album.name }}</h2><ArrowUpRight :size="18" aria-hidden="true" /></div>
        <p v-if="album.description" class="description">{{ album.description }}</p>
        <div class="card-meta">
          <span v-if="!mine" class="creator"><img class="avatar" :src="album.creator.avatar_url" alt="" />{{ album.creator.username }}</span>
          <span>{{ album.photo_count }} 张照片</span>
        </div>
      </div>
    </RouterLink>
  </div>
  <Pager v-if="data" :page="data" @change="changePage" />

  <dialog ref="dialog" class="modal" @cancel="saving && $event.preventDefault()">
    <form class="form-stack" @submit.prevent="createAlbum">
      <h2>新建相册</h2>
      <label>名称<input v-model="draft.name" autofocus required maxlength="60" placeholder="给相册起个名字" /></label>
      <label>描述 <span class="muted">（选填）</span><textarea v-model="draft.description" rows="3" maxlength="300" /></label>
      <fieldset>
        <legend>可见范围</legend>
        <label class="choice"><input v-model="draft.is_public" type="radio" :value="false" /><LockKeyhole :size="17" />私有，仅自己可见</label>
        <label class="choice"><input v-model="draft.is_public" type="radio" :value="true" /><Globe2 :size="17" />公开，出现在广场</label>
      </fieldset>
      <p v-if="formError" class="error-message" role="alert">{{ formError }}</p>
      <div class="form-actions">
        <button class="button secondary" type="button" :disabled="saving" @click="dialog.close()">取消</button>
        <button class="button primary" :disabled="saving">{{ saving ? '正在创建…' : '创建相册' }}</button>
      </div>
    </form>
  </dialog>
</template>
