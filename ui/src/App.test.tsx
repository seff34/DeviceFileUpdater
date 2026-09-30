import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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
  render(
    <QueryClientProvider client={qc}>
      <TooltipProvider>
        <WizardProvider>
          <App />
        </WizardProvider>
      </TooltipProvider>
    </QueryClientProvider>,
  )
  return qc
}

/** Waits until the gating queries have loaded, so the redirect effect saw real facts. */
async function settled(qc: QueryClient) {
  await waitFor(() => {
    expect(qc.getQueryState(['run'])?.status).toBe('success')
    expect(qc.isFetching()).toBe(0)
  })
}

const populated = {
  '/api/workspace': { current: '/ws', recent: ['/ws'] },
  '/api/devices': { devices: [{ host: '10.0.0.1', username: 'root', password: 'x' }] },
  '/api/manifest': {
    entries: [{ local_path: 'a.bin', remote_path: '/opt/a.bin', mode: '0644', file: { exists: true, size: 1, sha256: 'ab' } }],
  },
  '/api/settings': {
    parallel: 4, backup: true, post_command: '', post_command_policy: 'on_change',
    connect_timeout_sec: 10, command_timeout_sec: 60, strict_host_key: false,
  },
}

const run = (p: Record<string, unknown>) => ({
  state: 'idle', dry_run: false, only_failed_from: '', total_devices: 0,
  report_id: '', preview_id: '', preview_fresh: false, error: '', ...p,
})

const applied = run({ state: 'done', dry_run: false, total_devices: 1, report_id: 'r1', preview_id: 'p', preview_fresh: true })

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
  it('keeps a deep link to the report after a finished real run', async () => {
    window.history.replaceState(null, '', '/report')
    mockApi({ ...populated, '/api/runs/current': applied })
    const qc = renderApp()
    await settled(qc)
    expect(window.location.pathname).toBe('/report')
    expect(screen.getByRole('heading', { name: 'Rapor' })).toBeInTheDocument()
  })
  it('lands on the report after a finished real run', async () => {
    mockApi({ ...populated, '/api/runs/current': applied })
    const qc = renderApp()
    await settled(qc)
    expect(window.location.pathname).toBe('/report')
  })
  it('keeps a deep link to settings when earlier steps are valid', async () => {
    window.history.replaceState(null, '', '/settings')
    mockApi({ ...populated, '/api/runs/current': run({}) })
    const qc = renderApp()
    await settled(qc)
    expect(window.location.pathname).toBe('/settings')
    expect(screen.getByRole('heading', { name: 'Ayarlar' })).toBeInTheDocument()
  })
  it('opens the usage guide from the top bar and keeps it without redirecting', async () => {
    mockApi({ ...populated, '/api/runs/current': run({}) })
    const qc = renderApp()
    await settled(qc)
    screen.getByRole('link', { name: 'Kullanım kılavuzu' }).click()
    expect(await screen.findByRole('heading', { level: 1, name: 'Kullanım kılavuzu' })).toBeInTheDocument()
    await settled(qc)
    expect(window.location.pathname).toBe('/help')
    expect(screen.getAllByRole('link', { name: /Bu adıma git/ })).toHaveLength(7)
  })
  it('shows the session screen on 401', async () => {
    mockApi({ '/api/workspace': new Response(JSON.stringify({ error: 'Oturum bulunamadı.' }), { status: 401 }) })
    renderApp()
    expect(await screen.findByText('Oturum bulunamadı')).toBeInTheDocument()
  })
})
