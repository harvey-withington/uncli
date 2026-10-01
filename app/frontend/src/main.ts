import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { createBackend } from './lib/api'
import { applyTheme } from './lib/theme.svelte'

applyTheme()

const target = document.getElementById('app')
if (!target) throw new Error('#app missing')

export default mount(App, { target, props: { backend: createBackend() } })
