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
    // The cell explains the error in Turkish and keeps the raw engine text in its tooltip.
    const err = within(row2).getByText('Cihaz yanıt verdi ama SSH ve Telnet portu kapalı. IP ve portu kontrol edin.')
    expect(err).toHaveAttribute('title', 'connect: connection refused')
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
    // A failed device counts the files that succeeded, not the ones it gave up on.
    expect(within(screen.getByRole('row', { name: /10\.0\.0\.2/ })).getByText('0/2')).toBeInTheDocument()
    expect(within(screen.getByRole('row', { name: /10\.0\.0\.1/ })).getByText('2/2')).toBeInTheDocument()
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

  it('treats a 404 on cancel as already finished without an error toast', async () => {
    installFakeEventSource()
    setup({ 'DELETE /api/runs/current': () => json({ error: 'Süren bir çalışma yok.' }, 404) })
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await userEvent.click(await screen.findByRole('button', { name: 'İptal et' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Çalıştırmayı iptal et' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE')).toBe(true))
    await waitFor(() => expect(calls.filter((c) => c.method === 'GET' && c.url === '/api/runs/current').length).toBeGreaterThan(1))
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('toasts other cancel failures and keeps the button usable', async () => {
    installFakeEventSource()
    setup({ 'DELETE /api/runs/current': () => json({ error: 'sunucu hatası' }, 500) })
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await userEvent.click(await screen.findByRole('button', { name: 'İptal et' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Çalıştırmayı iptal et' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('sunucu hatası'))
    expect(screen.getByRole('button', { name: 'İptal et' })).toBeEnabled()
  })

  it('closes the confirm dialog when the run ends meanwhile', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await userEvent.click(await screen.findByRole('button', { name: 'İptal et' }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    run = status({ state: 'done', report_id: 'r1' })
    await act(async () =>
      ES.last!.emit({ type: 'run_done', done: 0, total: 0, report_id: 'r1', run: { id: 'r1', started: '', finished: '', dry_run: false, parallel: 1, files: [], devices: [] } }),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('refetches the run when the stream dies before run_done', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await waitFor(() => expect(ES.last).not.toBeNull())
    const runGets = () => calls.filter((c) => c.method === 'GET' && c.url === '/api/runs/current').length
    const before = runGets()
    await act(async () => ES.last!.fail())
    await waitFor(() => expect(runGets()).toBeGreaterThan(before))
  })

  it('shows Turkish text for cancelled devices of both server shapes', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await waitFor(() => expect(ES.last).not.toBeNull())
    await act(async () => {
      await Promise.resolve()
      ES.last!.emit({ type: 'device_state', host: '10.0.0.1', stage: 'failed', done: 0, total: 1, error: 'connect: context canceled' })
      ES.last!.emit({ type: 'device_state', host: '10.0.0.2', stage: 'syncing', done: 0, total: 1 })
      ES.last!.emit({ type: 'file_result', host: '10.0.0.2', done: 1, total: 1, file: { remote: '/etc/a', status: 'FAILED', error: 'cancelled', duration_ms: 0 } })
      await new Promise((r) => requestAnimationFrame(() => r(null)))
    })
    expect(within(screen.getByRole('row', { name: /10\.0\.0\.1/ })).getByText('İptal edildi')).toBeInTheDocument()
    expect(within(screen.getByRole('row', { name: /10\.0\.0\.2/ })).getByText('İptal edildi')).toBeInTheDocument()
    expect(screen.queryByText(/cancel(l)?ed$/)).not.toBeInTheDocument()
  })

  it('shows the real file error when a device has mixed real and cancelled failures', async () => {
    const ES = installFakeEventSource()
    renderWithProviders(<ApplyStep />, { path: '/apply' })
    await waitFor(() => expect(ES.last).not.toBeNull())
    await act(async () => {
      await Promise.resolve()
      ES.last!.emit({ type: 'device_state', host: '10.0.0.1', stage: 'syncing', done: 0, total: 2 })
      ES.last!.emit({ type: 'file_result', host: '10.0.0.1', done: 1, total: 2, file: { remote: '/etc/a', status: 'FAILED', error: 'permission denied', duration_ms: 0 } })
      ES.last!.emit({ type: 'file_result', host: '10.0.0.1', done: 2, total: 2, file: { remote: '/etc/b', status: 'FAILED', error: 'cancelled', duration_ms: 0 } })
      await new Promise((r) => requestAnimationFrame(() => r(null)))
    })
    const row = screen.getByRole('row', { name: /10\.0\.0\.1/ })
    expect(within(row).getByText('/etc/a: Cihazda bu yola yazma izni yok.')).toHaveAttribute('title', 'permission denied')
    expect(within(row).queryByText('İptal edildi')).not.toBeInTheDocument()
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
