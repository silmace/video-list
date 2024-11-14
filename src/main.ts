import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import router from './router'

// Vuetify
import 'vuetify/styles'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'
import { md3 } from 'vuetify/blueprints'
import '@mdi/font/css/materialdesignicons.css'

const vuetify = createVuetify({
  blueprint: md3,
  components,
  directives,
  theme: {
    defaultTheme: 'light',
    themes: {
      light: {
        dark: false,
        colors: {
          primary: '#4CAF50',
          secondary: '#FFC107', 
          background: '#F6F6F6',
          surface: '#FFFFFF',
          'surface-variant': '#E7E0EC',
          'on-surface-variant': '#49454F',
          error: '#B3261E',
          info: '#2196F3',
          success: '#4CAF50',
          warning: '#E91E63',
        }
      },
      dark: {
        dark: true,
        colors: {
          primary: '#D0BCFF',
          secondary: '#CCC2DC',
          background: '#1C1B1F',
          surface: '#2B2930',
          'surface-variant': '#49454F',
          'on-surface-variant': '#CAC4D0',
          error: '#F2B8B5',
          info: '#64B5F6',
          success: '#81C784',
          warning: '#FFB74D',
        }
      }
    }
  }
})

const app = createApp(App)
app.use(router)
app.use(vuetify)
app.mount('#app')