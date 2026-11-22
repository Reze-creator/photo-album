import { session, setSession } from './session'

export class RequestError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

// JSON 接口与授权图片共享同一层网络处理，上传时由浏览器生成 multipart 边界。
async function request(path, { method = 'GET', body, signal, auth = true, binary = false } = {}) {
  const headers = new Headers()
  const token = auth ? session.token : ''
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (body && !(body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(body)
  }

  let response
  try {
    response = await fetch(path, { method, body, headers, signal })
  } catch (error) {
    if (error.name === 'AbortError') throw error
    throw new RequestError(0, '连接失败，请检查服务是否已启动')
  }
  if (response.ok && binary) return response.blob()

  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.code !== 0) {
    if (response.status === 401 && token && token === session.token) setSession(null)
    const fallback = response.status === 413 ? '上传文件超过大小限制' : `请求失败（${response.status}）`
    throw new RequestError(response.status, payload?.message || fallback)
  }
  return payload.data
}

export const api = {
  register: (body) => request('/api/v1/register', { method: 'POST', body, auth: false }),
  login: (body) => request('/api/v1/login', { method: 'POST', body, auth: false }),
  albums: (mine, page, signal) => request(`/api/v1/albums${mine ? '' : '/public'}?page=${page}`, { signal, auth: mine }),
  createAlbum: (body) => request('/api/v1/albums', { method: 'POST', body }),
  album: (id, signal) => request(`/api/v1/albums/${id}`, { signal }),
  photos: (id, page, signal) => request(`/api/v1/albums/${id}/photos?page=${page}`, { signal }),
  uploadPhoto: (id, body) => request(`/api/v1/albums/${id}/photos`, { method: 'POST', body }),
  image: (path, signal) => request(path, { signal, binary: true }),
}

export function checkImage(file, maxMB) {
  if (!file) return '请选择一张图片'
  if (!/\.(jpe?g|png|gif)$/i.test(file.name)) return '请选择 JPG、PNG 或 GIF 图片'
  if (file.size === 0 || file.size > maxMB * 1024 * 1024) return `图片大小需大于 0 且不超过 ${maxMB} MB`
  return ''
}
