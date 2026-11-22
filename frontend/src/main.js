import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import AlbumList from './views/AlbumList.vue'
import AlbumDetail from './views/AlbumDetail.vue'
import AuthView from './views/AuthView.vue'
import NotFound from './views/NotFound.vue'
import { session } from './session'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'public', component: AlbumList, props: { mine: false } },
    { path: '/my-albums', name: 'mine', component: AlbumList, props: { mine: true }, meta: { login: true } },
    { path: '/albums/:id(\\d+)', name: 'album', component: AlbumDetail, props: true },
    { path: '/login', name: 'login', component: AuthView, props: { register: false } },
    { path: '/register', name: 'register', component: AuthView, props: { register: true } },
    { path: '/:pathMatch(.*)*', component: NotFound },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

// 路由守卫改善交互，真正的数据访问权限始终由后端判定。
router.beforeEach((to) => {
  if (to.meta.login && !session.token) return { name: 'login', query: { next: to.fullPath } }
})

createApp(App).use(router).mount('#app')
