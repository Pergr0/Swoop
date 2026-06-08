import './style.css'
import App from './App.svelte'
import { locale } from './i18n'

document.documentElement.lang = locale

const app = new App({
  target: document.getElementById('app')
})

export default app
