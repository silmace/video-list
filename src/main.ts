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
          primary: '#0f766e',
          secondary: '#d97706',
          background: '#f4efe6',
          surface: '#fffaf2',
          'surface-variant': '#efe4d2',
          'on-surface-variant': '#5d4a33',
          error: '#c2410c',
          info: '#0369a1',
          success: '#15803d',
          warning: '#b45309',
        }
      },
      dark: {
        dark: true,
        colors: {
          primary: '#7dd3c7',
          secondary: '#f6ad55',
          background: '#16120f',
          surface: '#211b17',
          'surface-variant': '#332923',
          'on-surface-variant': '#eadbc7',
          error: '#fdba74',
          info: '#7dd3fc',
          success: '#86efac',
          warning: '#fdba74',
        }
      }
    }
  }
})

const app = createApp(App)
app.use(router)
app.use(vuetify)
app.mount('#app')