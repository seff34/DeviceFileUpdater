import { beforeEach, describe, expect, it } from 'vitest'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PreviewStep } from './PreviewStep'
import { installFakeEventSource, mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { RunResult, RunStatus } from '@/lib/types'

const status = (p: Partial<RunStatus>): RunStatus => ({
  state: 'idle', dry_run: false, only_failed_from: '', total_devices: 0, report_id: '', preview_id: '', preview_fresh: false, error: '', ...p,
})
const report: RunResult = {
  id: '20260930-101010', started: '', finished: '', dry_run: true, parallel: 2, files: ['/etc/app.conf'],
  devices: [
    { host: '10.0.0.1', duration_ms: 1, files: [{ remote: '/etc/app.conf', status: 'WOULD_CREATE', duration_ms: 1 }] },
    { host: '10.0.0.2', duration_ms: 1, error: 'Cihaza ulaşılamadı: timeout', files: [{ remote: '/etc/app.conf', status: 'FAILED', duration_ms: 0 }] },
  ],
}

let run: RunStatus
let calls: Call[]
let ES: ReturnType<typeof installFakeEventSource>

function setup(extra: Record<string, unknown> = {}) {
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    '/api/devices': { devices: [{ host: '10.0.0.1', username: 'r', password: 'x' }, { host: '10.0.0.2', username: 'r', password: 'x' }] },
    '/api/manifest': { entries: [{ local_path: 'files/app.conf', remote_path: '/etc/app.conf', mode: '', file: { exists: true, size: 1, sha256: 'a' } }] },
    '/api/settings': { parallel: 2 },
    'GET /api/runs/current': () => run,
    'POST /api/runs': ({ body }: Call) => {
      const b = body as { dry_run: boolean; preview_id?: string }
      run = status({ state: 'running', dry_run: b.dry_run, total_devices: 2, preview_id: run.preview_id, preview_fresh: run.preview_fresh })
      return run
    },
    [`/api/reports/${report.id}`]: report,
    ...extra,
  })
}

beforeEach(() => {
  ES = installFakeEventSource()
})

describe('PreviewStep', () => {
  it('runs a dry-run with live progress and shows the matrix when done', async () => {
    run = status({})
    setup()
    renderWithProviders(<PreviewStep />, { path: '/preview' })
    await userEvent.click(await screen.findByRole('button', { name: 'Önizlemeyi başlat' }))
    await waitFor(() => expect(ES.last).not.toBeNull())
    await act(async () => {
      await Promise.resolve()
      ES.last!.emit({ type: 'device_state', host: '10.0.0.1', stage: 'done', done: 1, total: 1 })
    })
    expect(await screen.findByText('1/2 cihaz tamamlandı')).toBeInTheDocument()
    run = status({ state: 'done', dry_run: true, report_id: report.id, preview_id: report.id, preview_fresh: true })
    await act(async () => ES.last!.emit({ type: 'run_done', done: 2, total: 2, run: report, report_id: report.id }))
    expect(await screen.findByText('1 cihazda 1 dosya oluşturulacak. 1 cihaza ulaşılamadı.')).toBeInTheDocument()
    expect(screen.getByText('Oluşturulacak')).toBeInTheDocument()
    expect(screen.getByText(/Cihaza ulaşılamadı: timeout/)).toBeInTheDocument()
  })

  it('refetches the run status when the stream dies before run_done', async () => {
    run = status({})
    setup()
    renderWithProviders(<PreviewStep />, { path: '/preview' })
    await userEvent.click(await screen.findByRole('button', { name: 'Önizlemeyi başlat' }))
    await waitFor(() => expect(ES.last).not.toBeNull())
    const before = calls.filter((c) => c.url === '/api/runs/current' && c.method === 'GET').length
    await act(async () => {
      await Promise.resolve()
      ES.last!.close()
      ES.last!.onerror?.()
    })
    await waitFor(() => expect(calls.filter((c) => c.url === '/api/runs/current' && c.method === 'GET').length).toBeGreaterThan(before))
  })

  it('requires explicit confirmation, then starts the real run with the preview id', async () => {
    run = status({ state: 'done', dry_run: true, report_id: report.id, preview_id: report.id, preview_fresh: true })
    setup()
    renderWithProviders(<PreviewStep />, { path: '/preview' })
    expect(await screen.findByText('Değişiklikleri onaylayın.')).toBeInTheDocument()
    await userEvent.click(await screen.findByRole('checkbox', { name: /Değişiklikleri inceledim/ }))
    await userEvent.click(screen.getByRole('button', { name: /Uygulamayı başlat/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.url === '/api/runs')).toBe(true))
    expect(calls.find((c) => c.method === 'POST')!.body).toEqual({ dry_run: false, preview_id: report.id })
    await waitFor(() => expect(window.location.pathname).toBe('/apply'))
  })

  it('warns when the preview is stale', async () => {
    run = status({ state: 'done', dry_run: true, report_id: report.id, preview_id: report.id, preview_fresh: false })
    setup()
    renderWithProviders(<PreviewStep />, { path: '/preview' })
    expect(await screen.findByText(/Önizlemeden sonra cihazlar, dosyalar veya ayarlar değişti/)).toBeInTheDocument()
    expect(screen.getByText('Önce güncel bir önizleme çalıştırın.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Önizlemeyi yeniden çalıştır' })).toBeEnabled()
  })

  it('keeps the start button disabled and explains when devices fail to load', async () => {
    run = status({})
    setup({ '/api/devices': () => new Response(JSON.stringify({ error: 'boom' }), { status: 500 }) })
    renderWithProviders(<PreviewStep />, { path: '/preview' })
    expect(await screen.findByText('Cihazlar yüklenemedi.', { selector: 'p' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Önizlemeyi başlat' })).toBeDisabled()
  })
})
