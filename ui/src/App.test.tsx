import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TooltipProvider } from '@/components/ui/tooltip'
import { WizardProvider } from '@/wizard/WizardContext'
import App from './App'

function mockApi(routes: Record<string, unknown>) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : (input as Request).url
    const key = Object.keys(routes).find((k) => url.startsWith(k))
    if (!key) return new Response(JSON.stringify({ error: 'nope' }), { status: 404 })
    const v = routes[key]
    return v instanceof Response ? v : new Response(JSON.stringify(v), { status: 200 })
  })
}

function renderApp() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <TooltipProvider>
        <WizardProvider>
          <App />
        </WizardProvider>
      </TooltipProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.restoreAllMocks()
  window.history.replaceState(null, '', '/')
})

describe('App', () => {
  it('lands on the workspace step when none is open', async () => {
    mockApi({ '/api/workspace': { current: '', recent: [] } })
    renderApp()
    expect(await screen.findByRole('heading', { name: 'Çalışma alanı' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/workspace')
  })
  it('shows the session screen on 401', async () => {
    mockApi({ '/api/workspace': new Response(JSON.stringify({ error: 'Oturum bulunamadı.' }), { status: 401 }) })
    renderApp()
    expect(await screen.findByText('Oturum bulunamadı')).toBeInTheDocument()
  })
})
