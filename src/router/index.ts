import { createRouter, createWebHistory,createWebHashHistory } from 'vue-router'
import FileList from '../components/FileList.vue'
import VideoEditor from '../components/VideoEditor.vue'

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: FileList
    },
    {
      path: '/edit',
      component: VideoEditor
    }
  ]
})

export default router