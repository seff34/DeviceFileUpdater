import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { WizardProvider } from '@/wizard/WizardContext'
import App from './App'
import './index.css'

// Follow the OS theme: shadcn's tokens switch on the .dark class.
const mq = window.matchMedia('(prefers-color-scheme: dark)')
const applyTheme = () => document.documentElement.classList.toggle('dark', mq.matches)
applyTheme()
mq.addEventListener('change', applyTheme)

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: (n, err) => (err as { status?: number }).status !== 401 && n < 2, refetchOnWindowFocus: false },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={300}>
        <WizardProvider>
          <App />
        </WizardProvider>
        <Toaster position="top-center" offset={64} />
      </TooltipProvider>
    </QueryClientProvider>
  </StrictMode>,
)
