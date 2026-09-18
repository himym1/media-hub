import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { bindInAppLinks } from './shared/desktop/inAppBrowser'
import { markDesktopShell } from './shared/desktop/desktopShell'
import './shared/desktop/desktop-shell.css'

markDesktopShell()
bindInAppLinks()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
