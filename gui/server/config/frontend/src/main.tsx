import React, { useEffect, useMemo, useState } from 'react'
import ReactDOM from 'react-dom/client'
// Mantine styles must be imported before the app styles; notifications styles
// must come after core styles (see https://mantine.dev/getting-started/).
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { Events } from '@wailsio/runtime'
import { SystemService } from '../bindings/changeme'
import App from './App'
import { createAppTheme, cssVariablesResolver } from './theme'

/**
 * The interface follows the desktop it runs on: `auto` takes the light or dark
 * scheme straight from the system, and the accent colour is read from the Go
 * side at startup, when the desktop reports a theme change and when the window
 * comes back to the front.
 */
function Root() {
  const [accent, setAccent] = useState('')

  useEffect(() => {
    const readAccent = () => {
      SystemService.AccentColour()
        .then((colour) => setAccent(colour ?? ''))
        .catch(() => setAccent(''))
    }
    readAccent()
    // The desktop reports theme changes while the window is open. Windows does
    // not report an accent change on its own, and the usual way of changing it
    // is to leave the window for the system settings and come back, so reading
    // the accent again on focus keeps the interface up to date.
    window.addEventListener('focus', readAccent)
    const stopListening = Events.On('system-theme-changed', readAccent)
    return () => {
      window.removeEventListener('focus', readAccent)
      stopListening()
    }
  }, [])

  const theme = useMemo(() => createAppTheme(accent), [accent])

  return (
    <MantineProvider
      theme={theme}
      defaultColorScheme="auto"
      cssVariablesResolver={cssVariablesResolver}
    >
      <Notifications position="bottom-right" limit={3} />
      <App />
    </MantineProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
)
