import { createRouter, createWebHistory } from 'vue-router'
import FileList from '../components/FileList.vue'
import VideoEditor from '../components/VideoEditor.vue'
import LoginView from '../views/LoginView.vue'
import SettingsView from '../views/SettingsView.vue'
import TasksView from '../views/TasksView.vue'
import { authState, checkAuthStatus } from '../composables/useAuth'

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