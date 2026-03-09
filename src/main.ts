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
          primary: '#1A73E8',
          secondary: '#0EA5A4', 
          background: '#F5FBFF',
          surface: '#FFFFFF',
          'surface-variant': '#D9EBFA',
          'on-surface-variant': '#23405D',
          error: '#B3261E',
          info: '#2196F3',
          success: '#27AE60',
          warning: '#F2994A',
        }
      },
      dark: {
        dark: true,
        colors: {
          primary: '#89C2FF',
          secondary: '#6DE9D5',
          background: '#101820',
          surface: '#182430',
          'surface-variant': '#263748',
          'on-surface-variant': '#D5E6F5',
          error: '#F2B8B5',
          info: '#64B5F6',
          success: '#6FCF97',
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