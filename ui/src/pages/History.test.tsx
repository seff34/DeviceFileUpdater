import { describe, expect, it, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { History, ReportPage } from './History'
import { json, mockApi, renderWithProviders } from '@/test/utils'

beforeEach(() => vi.restoreAllMocks())

const ws = { '/api/workspace': { current: '/srv/ws', recent: [] } }

describe('History', () => {
  it('lists reports newest first and opens one', async () => {
    mockApi({
      ...ws,
      '/api/reports': {
        reports: [
          { id: '20260930-130000', started: '2026-09-30T13:00:00Z', finished: '2026-09-30T13:02:00Z', dry_run: false, devices: 40, devices_failed: 3, by_status: { UPDATED: 30, FAILED: 3 } },
          { id: '20260930-120000', started: '2026-09-30T12:00:00Z', finished: '2026-09-30T12:01:00Z', dry_run: true, devices: 40, devices_failed: 0, by_status: { WOULD_UPDATE: 30 } },
        ],
      },
    })
    renderWithProviders(<History />, { path: '/history' })
    const rows = await screen.findAllByRole('link', { name: /20260930-/ })
    expect(rows[0]).toHaveTextContent('20260930-130000')
    expect(screen.getByText(/3 başarısız/)).toBeInTheDocument()
    expect(screen.getByText('Önizleme')).toBeInTheDocument()
    await userEvent.click(rows[1])
    await waitFor(() => expect(window.location.pathname).toBe('/history/20260930-120000'))
  })
  it('asks for a workspace first', async () => {
    mockApi({ '/api/workspace': { current: '', recent: [] } })
    renderWithProviders(<History />, { path: '/history' })
    expect(await screen.findByText('Raporları görmek için önce bir çalışma alanı açın.')).toBeInTheDocument()
  })
  it('shows an empty state', async () => {
    mockApi({ ...ws, '/api/reports': { reports: [] } })
    renderWithProviders(<History />, { path: '/history' })
    expect(await screen.findByText(/henüz rapor yok/)).toBeInTheDocument()
  })
  it('shows an error alert when the list fails', async () => {
    mockApi({ ...ws, '/api/reports': () => json({ error: 'okunamadı' }, 500) })
    renderWithProviders(<History />, { path: '/history' })
    expect(await screen.findByRole('alert')).toHaveTextContent('okunamadı')
  })
  it('shows an error alert for a missing report', async () => {
    mockApi({ ...ws, '/api/reports/x': () => json({ error: 'Rapor bulunamadı' }, 404) })
    renderWithProviders(<ReportPage id="x" />, { path: '/history/x' })
    expect(await screen.findByRole('alert')).toHaveTextContent('Rapor bulunamadı')
  })
})
