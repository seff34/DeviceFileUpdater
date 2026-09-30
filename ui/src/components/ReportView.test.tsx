import { describe, expect, it, vi, beforeEach } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { ReportView } from './ReportView'
import { json, mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { RunResult } from '@/lib/types'

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn() }) }))

const result: RunResult = {
  id: '20260930-120000-ab12', started: '2026-09-30T12:00:00Z', finished: '2026-09-30T12:01:05Z', dry_run: false, parallel: 4,
  files: ['/etc/a.conf', '/etc/b.conf'],
  devices: [
    { host: '10.0.0.1', protocol: 'ssh', upload_method: 'sftp', duration_ms: 4200, files: [{ remote: '/etc/a.conf', status: 'UPDATED', method: 'sftp', duration_ms: 900 }, { remote: '/etc/b.conf', status: 'UNCHANGED', duration_ms: 5 }], post: { command: 'reboot-app', exit_code: 0, output: 'ok line 1\nok line 2' } },
    { host: '10.0.0.2', protocol: 'telnet', upload_method: 'shell-printf', duration_ms: 9000, files: [{ remote: '/etc/a.conf', status: 'FAILED', duration_ms: 10, error: 'permission denied' }, { remote: '/etc/b.conf', status: 'CREATED', duration_ms: 50 }] },
  ],
}
let calls: Call[]

beforeEach(() => {
  vi.clearAllMocks()
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    '/api/devices': { devices: [{ host: '10.0.0.1', username: 'u', password: '' }, { host: '10.0.0.2', username: 'u', password: '' }] },
    '/api/runs/current': { state: 'done', dry_run: false },
    'POST /api/runs': { state: 'running', dry_run: false, only_failed_from: result.id, total_devices: 1 },
  })
})

describe('ReportView', () => {
  it('summarises, filters failures and expands device files', async () => {
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />)
    expect(screen.getByText('2 cihazdan 1 tanesi başarılı, 1 tanesi başarısız. 1 dosya oluşturuldu, 1 dosya güncellendi, 1 dosya aynıydı, 1 dosya başarısız.')).toBeInTheDocument()
    const open = screen.getByRole('link', { name: /HTML raporu aç/ })
    expect(open).toHaveAttribute('href', `/api/reports/${result.id}/html`)
    expect(open).toHaveAttribute('target', '_blank')
    expect(open).toHaveAttribute('rel', expect.stringContaining('noopener'))
    expect(screen.getByRole('link', { name: /İndir/ })).toHaveAttribute('href', `/api/reports/${result.id}/html?download=1`)
    await userEvent.click(screen.getByRole('button', { name: /Sadece başarısızlar/ }))
    expect(screen.queryByText('10.0.0.1')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /10\.0\.0\.2/ }))
    expect(await screen.findByText('permission denied')).toBeInTheDocument()
  })
  it('shows post-command output in a scrollable capped block', async () => {
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />)
    await userEvent.click(screen.getByRole('button', { name: /10\.0\.0\.1/ }))
    const out = await screen.findByText(/ok line 1/)
    expect(out.tagName).toBe('PRE')
    expect(out).toHaveClass('max-h-40', 'overflow-auto', 'font-mono')
  })
  it('shows cancelled devices as Turkish text, never raw English', async () => {
    const r: RunResult = {
      ...result,
      devices: [
        { host: '10.0.0.5', duration_ms: 1, error: 'connect: context canceled', files: [] },
        { host: '10.0.0.6', duration_ms: 1, files: [{ remote: '/etc/a.conf', status: 'FAILED', duration_ms: 0, error: 'cancelled' }] },
        { host: '10.0.0.7', duration_ms: 1, error: 'dial tcp: operation was canceled', files: [] },
      ],
    }
    renderWithProviders(<ReportView result={r} reportId={r.id} allowRetry />)
    expect(screen.getAllByText('İptal edildi')).toHaveLength(3)
    await userEvent.click(screen.getByRole('button', { name: /10\.0\.0\.6/ }))
    expect(screen.getAllByText('İptal edildi')).toHaveLength(4)
    expect(screen.queryByText(/cancel/i)).not.toBeInTheDocument()
  })
  it('retries only failed devices after confirmation', async () => {
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/1 cihaz/)).toBeInTheDocument()
    expect(within(dialog).getByText(/10\.0\.0\.2/)).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Tekrar dene' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST')).toBe(true))
    expect(calls.find((c) => c.method === 'POST')!.body).toEqual({ dry_run: false, only_failed_from: result.id })
    await waitFor(() => expect(window.location.pathname).toBe('/apply'))
  })
  it.each([
    [409, 'Başka bir çalışma alanında işlem sürüyor.'],
    [422, 'Cihaz listesi geçersiz: satır 3.'],
    [422, 'Bu raporda başarısız cihaz yok.'],
    [404, 'Rapor bulunamadı: x'],
  ])('toasts the server message for %i and stays on the page', async (status, msg) => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [{ host: '10.0.0.2', username: 'u', password: '' }] },
      '/api/runs/current': { state: 'done', dry_run: false },
      'POST /api/runs': () => json({ error: msg }, status),
    })
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    await userEvent.click(await within(await screen.findByRole('dialog')).findByRole('button', { name: 'Tekrar dene' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(msg))
    expect(window.location.pathname).toBe('/report')
    expect(screen.getByRole('button', { name: /Başarısızları tekrar dene/ })).toBeEnabled()
  })
  it('falls back to Turkish text when the error message is empty', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [{ host: '10.0.0.2', username: 'u', password: '' }] },
      '/api/runs/current': { state: 'done', dry_run: false },
      'POST /api/runs': () => json({ error: '' }, 409),
    })
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    await userEvent.click(await within(await screen.findByRole('dialog')).findByRole('button', { name: 'Tekrar dene' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/çalışan bir işlem/)))
  })
  it('counts only failed hosts still in devices.csv and says how many are skipped', async () => {
    const r: RunResult = { ...result, devices: [...result.devices, { host: '10.0.0.3', duration_ms: 1, error: 'timeout', files: [] }] }
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [{ host: '10.0.0.2', username: 'u', password: '' }] },
      '/api/runs/current': { state: 'done', dry_run: false },
    })
    renderWithProviders(<ReportView result={r} reportId={r.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(/Yalnızca 1 cihazda/)).toBeInTheDocument()
    expect(within(dialog).getByText(/1 cihaz artık devices\.csv'de olmadığı için atlanacak/)).toBeInTheDocument()
    expect(within(dialog).getByText(/güncel dosya listesini ve ayarları/)).toBeInTheDocument()
  })
  it('disables confirm when no failed host remains in devices.csv', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [{ host: '10.0.0.1', username: 'u', password: '' }] },
      '/api/runs/current': { state: 'done', dry_run: false },
    })
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByText(/hiçbiri artık devices\.csv içinde değil/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Tekrar dene' })).toBeDisabled()
  })
  it('keeps confirm disabled while devices load', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': () => new Promise(() => {}),
      '/api/runs/current': { state: 'done', dry_run: false },
    })
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Cihaz listesi yükleniyor.')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Tekrar dene' })).toBeDisabled()
  })
  it('surfaces the real error when a device has mixed real and cancelled failures', async () => {
    const r: RunResult = {
      ...result,
      devices: [{ host: '10.0.0.8', duration_ms: 1, files: [
        { remote: '/etc/a.conf', status: 'FAILED', duration_ms: 0, error: 'permission denied' },
        { remote: '/etc/b.conf', status: 'FAILED', duration_ms: 0, error: 'cancelled' },
      ] }],
    }
    renderWithProviders(<ReportView result={r} reportId={r.id} allowRetry />)
    expect(screen.queryByText('İptal edildi')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /10\.0\.0\.8/ }))
    expect(screen.getByText('permission denied')).toBeInTheDocument()
  })
  it('keeps a real device error visible even when files were cancelled', () => {
    const r: RunResult = { ...result, devices: [{ host: '10.0.0.9', duration_ms: 1, error: 'auth failed', files: [{ remote: '/a', status: 'FAILED', duration_ms: 0, error: 'cancelled' }] }] }
    renderWithProviders(<ReportView result={r} reportId={r.id} allowRetry />)
    expect(screen.getByText('auth failed')).toBeInTheDocument()
    expect(screen.queryByText('İptal edildi')).not.toBeInTheDocument()
  })
  it('disables retry while the request is in flight', async () => {
    let release!: () => void
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [{ host: '10.0.0.1', username: 'u', password: '' }, { host: '10.0.0.2', username: 'u', password: '' }] },
      '/api/runs/current': { state: 'done', dry_run: false },
      'POST /api/runs': () => new Promise((res) => { release = () => res({ state: 'running', dry_run: false, only_failed_from: result.id }) }),
    })
    renderWithProviders(<ReportView result={result} reportId={result.id} allowRetry />, { path: '/report' })
    await userEvent.click(screen.getByRole('button', { name: /Başarısızları tekrar dene/ }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Tekrar dene' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Başarısızları tekrar dene/ })).toBeDisabled())
    release()
    await waitFor(() => expect(window.location.pathname).toBe('/apply'))
  })
  it('hides retry for a dry run', () => {
    renderWithProviders(<ReportView result={{ ...result, dry_run: true }} reportId={result.id} allowRetry />)
    expect(screen.queryByRole('button', { name: /Başarısızları tekrar dene/ })).not.toBeInTheDocument()
    expect(screen.getByText('Önizleme')).toBeInTheDocument()
  })
})
