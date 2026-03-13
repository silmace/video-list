import { createRouter, createWebHistory } from 'vue-router'
import { authState, checkAuthStatus } from '../composables/useAuth'

const FileList = () => import('../components/FileList.vue')
const VideoEditor = () => import('../components/VideoEditor.vue')
const LoginView = () => import('../views/LoginView.vue')
const SettingsView = () => import('../views/SettingsView.vue')
const TasksView = () => import('../views/TasksView.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      component: LoginView,
      meta: { public: true }
    },
    {
      path: '/settings',
      component: SettingsView
    },
    {
      path: '/tasks',
      component: TasksView
    },
    {
      path: '/edit/:pathMatch(.*)*',
      name: 'video-editor',
      component: VideoEditor,
      props: true
    },
    {
      path: '/',
      component: FileList
    },
    {
      path: '/:pathMatch(.*)*',
      component: FileList
    }
  ]
})

router.beforeEach(async (to) => {
  try {
    await checkAuthStatus()
  } catch {
    if (!to.meta.public) {
      return '/login'
    }
    return true
  }
  if (to.meta.public) {
    if (authState.authEnabled.value && authState.authenticated.value && to.path === '/login') {
      return '/'
    }
    return true
  }

  if (authState.authEnabled.value && !authState.authenticated.value) {
    return '/login'
  }

  return true
})

export default router