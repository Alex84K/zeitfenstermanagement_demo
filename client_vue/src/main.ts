import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'bootstrap/dist/css/bootstrap.min.css'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
