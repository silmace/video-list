import { createRouter, createWebHistory } from 'vue-router'
import FileList from '../components/FileList.vue'
import VideoEditor from '../components/VideoEditor.vue'

const router = createRouter({
  history: createWebHistory(),
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