<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowLeft, Globe2, ImagePlus, LockKeyhole } from '@lucide/vue'
import { api, checkImage } from '../api'
import { session } from '../session'
import { useLoader } from '../useLoader'
import AlbumImage from '../components/AlbumImage.vue'
import ContentState from '../components/ContentState.vue'
import Pager from '../components/Pager.vue'

const props = defineProps({ id: { type: String, required: true } })
const page = ref(1)
const { data, loading, error, load } = useLoader(async (id, currentPage, signal) => {
  const [album, photos] = await Promise.all([api.album(id, signal), api.photos(id, currentPage, signal)])
  return { album, photos }
})
const owner = computed(() => !!session.user && session.user.id === data.value?.album.creator.id)
const fileInput = ref(null)
const uploading = ref(false)
const uploadError = ref('')
const uploadSuccess = ref('')

function reload() { return load(props.id, page.value) }

watch(() => [props.id, session.token], () => {
  page.value = 1
  uploadError.value = ''
  uploadSuccess.value = ''
  reload()
}, { immediate: true })

function changePage(next) {
  page.value = next
  reload()
}

async function upload(event) {
  const file = event.target.files[0]
  event.target.value = ''
  if (!file || uploading.value) return
  uploadSuccess.value = ''
  uploadError.value = checkImage(file, 10)
  if (uploadError.value) return

  const albumId = props.id
  const body = new FormData()
  body.append('photo', file)
  uploading.value = true
  try {
    await api.uploadPhoto(albumId, body)
    // 用户可能在等待上传时打开另一册，只更新发起上传的相册页面。
    if (props.id === albumId) {
      uploadSuccess.value = '照片已上传'
      changePage(1)
    }
  } catch (reason) {
    if (props.id === albumId) uploadError.value = reason.message
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <RouterLink class="back-link" :to="owner ? '/my-albums' : '/'" ><ArrowLeft :size="17" />{{ owner ? '我的相册' : '公开广场' }}</RouterLink>
  <ContentState v-if="loading || error" :loading="loading" :error="error" @retry="reload">
    <RouterLink v-if="error?.status === 401 && !session.token" class="button primary" :to="{ name: 'login', query: { next: `/albums/${id}` } }">去登录</RouterLink>
  </ContentState>
  <template v-else-if="data">
    <section class="page-heading detail-heading">
      <div>
        <div class="album-detail-meta">
          <span class="creator"><img class="avatar" :src="data.album.creator.avatar_url" alt="" />{{ data.album.creator.username }}</span>
          <span class="privacy-chip inline-chip">
            <Globe2 v-if="data.album.is_public" :size="14" /><LockKeyhole v-else :size="14" />
            {{ data.album.is_public ? '公开' : '私有' }}
          </span>
          <span>{{ data.photos.total }} 张照片</span>
        </div>
        <h1>{{ data.album.name }}</h1>
        <p v-if="data.album.description">{{ data.album.description }}</p>
      </div>
      <div v-if="owner" class="upload-control">
        <input ref="fileInput" type="file" accept=".jpg,.jpeg,.png,.gif" hidden @change="upload" />
        <button class="button primary" type="button" :disabled="uploading" @click="fileInput.click()"><ImagePlus :size="19" />{{ uploading ? '正在上传…' : '上传照片' }}</button>
        <small>JPG、PNG 或 GIF，最大 10 MB</small>
      </div>
    </section>
    <p v-if="uploadError" class="error-message" role="alert">{{ uploadError }}</p>
    <p v-if="uploadSuccess" class="success-message" role="status">{{ uploadSuccess }}</p>
    <ContentState v-if="!data.photos.items.length" title="相册里还没有照片" />
    <div v-else class="photo-grid">
      <figure v-for="photo in data.photos.items" :key="photo.id" class="photo-tile">
        <AlbumImage :path="photo.file_url" :protected-image="!data.album.is_public" :alt="`${data.album.name}的照片`" />
      </figure>
    </div>
    <Pager :page="data.photos" @change="changePage" />
  </template>
</template>
