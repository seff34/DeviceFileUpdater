import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactElement } from 'react'
import { vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { WizardProvider } from '@/wizard/WizardContext'

export interface Call { method: string; url: string; body: unknown }
type Handler = unknown | ((req: Call) => unknown | Promise<unknown>)

export const json = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } })

/** Replaces fetch. Unknown routes answer 404. Returns the live call log. */
export function mockApi(routes: Record<string, Handler>): Call[] {
  const calls: Call[] = []
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    let body: unknown = init?.body
    if (body instanceof FormData) {
      const entries: Record<string, unknown> = {}
      body.forEach((v, k) => {
        const val = v instanceof File ? v.name : v
        const prev = entries[k]
        entries[k] = prev === undefined ? val : Array.isArray(prev) ? [...prev, val] : [prev, val]
      })
      body = entries
    } else if (typeof body === 'string') {
      try {
        body = JSON.parse(body)
      } catch {
        /* raw text body (CSV import) */
      }
    }
    const call = { method, url, body }
    calls.push(call)
    const path = url.split('?')[0]
    const h = routes[`${method} ${path}`] ?? routes[path]
    if (h === undefined) return json({ error: `no mock for ${method} ${path}` }, 404)
    const v = typeof h === 'function' ? await (h as (r: Call) => unknown)(call) : h
    return v instanceof Response ? v : json(v)
  })
  return calls
}

export function renderWithProviders(ui: ReactElement, { path = '/' }: { path?: string } = {}) {
  window.history.replaceState(null, '', path)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const r = render(
    <QueryClientProvider client={qc}>
      <TooltipProvider>
        <WizardProvider>{ui}</WizardProvider>
      </TooltipProvider>
    </QueryClientProvider>,
  )
  return { ...r, qc }
}
