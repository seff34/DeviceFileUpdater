import { describe, expect, it, vi, beforeEach } from 'vitest'
import { toast } from 'sonner'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApplyStep } from './ApplyStep'
import { installFakeEventSource, json, mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { RunResult, RunStatus } from '@/lib/types'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn() }) }))

const status = (p: Partial<RunStatus>): RunStatus => ({
  state: 'running', dry_run: false, only_failed_from: '', total_devices: 2, report_id: '', preview_id: 'p', preview_fresh: true, error: '', ...p,
})
let run: RunStatus
let calls: Call[]

function setup(extra: Record<string, unknown> = {}) {
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    '/api/devices': { devices: [{ host: '10.0.0.1', username: 'r', password: 'x' }, { host: '10.0.0.2', username: 'r', password: 'x' }] },
    '/api/manifest': { entries: [] },
    '/api/settings': {},
    'GET /api/runs/current': () => run,
    'DELETE /api/runs/current': () => new Response(null, { status: 204 }),
    '/api/reports': { reports: [] },
    ...extra,
  })
}

beforeEach(() => {
  vi.mocked(toast).mockClear()
  vi.mocked(toast.error).mockClear()
  run = status({})
  setup()
})

describe('ApplyStep', () => {
  it('shows each device stage live and the result when done', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await waitFor(() => expect(ES.last).not.toBeNull())
    await act(async () => {
      await Promise.resolve()
      ES.last!.emit({ type: 'device_state', host: '10.0.0.1', stage: 'syncing', done: 0, total: 2 })
      ES.last!.emit({ type: 'file_result', host: '10.0.0.1', done: 1, total: 2, file: { remote: '/etc/a', status: 'CREATED', duration_ms: 3 } })
      ES.last!.emit({ type: 'device_state', host: '10.0.0.2', stage: 'failed', done: 0, total: 2, error: 'connect: connection refused' })
      await new Promise((r) => requestAnimationFrame(() => r(null)))
    })
    const row1 = screen.getByRole('row', { name: /10\.0\.0\.1/ })
    expect(within(row1).getByText('Dosyalar işleniyor')).toBeInTheDocument()
    expect(within(row1).getByText('1/2')).toBeInTheDocument()
    const row2 = screen.getByRole('row', { name: /10\.0\.0\.2/ })
    expect(within(row2).getByText('Başarısız')).toBeInTheDocument()
    expect(within(row2).getByText('connect: connection refused')).toBeInTheDocument()
    expect(screen.getByText('Uygulama sürüyor.')).toBeInTheDocument()

    const result: RunResult = {
      id: 'r1', started: '', finished: '', dry_run: false, parallel: 2, files: ['/etc/a', '/etc/b'],
      devices: [
        { host: '10.0.0.1', duration_ms: 5, files: [{ remote: '/etc/a', status: 'CREATED', duration_ms: 3 }, { remote: '/etc/b', status: 'UNCHANGED', duration_ms: 1 }] },
        { host: '10.0.0.2', duration_ms: 5, error: 'connect: connection refused', files: [{ remote: '/etc/a', status: 'FAILED', duration_ms: 0 }, { remote: '/etc/b', status: 'FAILED', duration_ms: 0 }] },
      ],
    }
    run = status({ state: 'done', report_id: 'r1' })
    await act(async () => ES.last!.emit({ type: 'run_done', done: 0, total: 0, run: result, report_id: 'r1' }))
    expect(await screen.findByText('2 cihazdan 1 tanesi başarılı, 1 tanesi başarısız. 1 dosya oluşturuldu, 1 dosya aynıydı, 2 dosya başarısız.')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: /Raporu gör/ })).toBeEnabled())
    expect(screen.queryByRole('button', { name: 'İptal et' })).not.toBeInTheDocument()
  })

  it('cancels only after confirmation', async () => {
    installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await userEvent.click(await screen.findByRole('button', { name: 'İptal et' }))
    const dialog = await screen.findByRole('dialog')
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    await userEvent.click(within(dialog).getByRole('button', { name: 'Çalıştırmayı iptal et' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE')).toBe(true))
    await waitFor(() => expect(toast).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'İptal et' })).toBeDisabled()
  })

  it('reports a cancel failure with a toast and keeps the button usable', async () => {
    installFakeEventSource()
    setup({ 'DELETE /api/runs/current': () => json({ error: 'çalışan bir işlem yok' }, 409) })
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await userEvent.click(await screen.findByRole('button', { name: 'İptal et' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Çalıştırmayı iptal et' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('çalışan bir işlem yok'))
    expect(screen.getByRole('button', { name: 'İptal et' })).toBeEnabled()
  })

  it('shows cancelled devices as cancelled and filters failed rows', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await waitFor(() => expect(ES.last).not.toBeNull())
    await act(async () => {
      await Promise.resolve()
      ES.last!.emit({ type: 'device_state', host: '10.0.0.1', stage: 'done', done: 1, total: 1 })
      ES.last!.emit({ type: 'device_state', host: '10.0.0.2', stage: 'failed', done: 0, total: 1, error: 'cancelled' })
      await new Promise((r) => requestAnimationFrame(() => r(null)))
    })
    expect(screen.getByText('İptal edildi')).toHaveAttribute('title', 'İptal edildi')
    await userEvent.click(screen.getByRole('button', { name: /^Başarısız 1$/ }))
    expect(screen.queryByRole('row', { name: /10\.0\.0\.1/ })).not.toBeInTheDocument()
    expect(screen.getByRole('row', { name: /10\.0\.0\.2/ })).toBeInTheDocument()
  })

  it('does not show a dry run and shows only the failed devices of a retry', async () => {
    run = status({ state: 'running', dry_run: true })
    const ES = installFakeEventSource()
    const { unmount } = renderWithProviders(<ApplyStep />, { path: '/apply' })
    expect(await screen.findByText('Şu anda çalışan bir uygulama yok.')).toBeInTheDocument()
    expect(ES.last).toBeNull()
    unmount()

    run = status({ only_failed_from: 'old' })
    setup({
      '/api/reports/old': {
        id: 'old', started: '', finished: '', dry_run: false, parallel: 2, files: ['/a'],
        devices: [
          { host: '10.0.0.1', duration_ms: 1, files: [{ remote: '/a', status: 'CREATED', duration_ms: 1 }] },
          { host: '10.0.0.2', duration_ms: 1, error: 'x', files: [] },
        ],
      },
    })
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    expect(await screen.findByRole('row', { name: /10\.0\.0\.2/ })).toBeInTheDocument()
    expect(screen.queryByRole('row', { name: /10\.0\.0\.1/ })).not.toBeInTheDocument()
  })

  it('shows an error when the device list cannot load', async () => {
    installFakeEventSource()
    setup({ '/api/devices': () => json({ error: 'liste okunamadı' }, 500) })
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    expect(await screen.findByText('liste okunamadı')).toBeInTheDocument()
  })
})
