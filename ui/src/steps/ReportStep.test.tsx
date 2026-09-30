import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { ReportStep } from './ReportStep'
import { mockApi, renderWithProviders } from '@/test/utils'

describe('ReportStep', () => {
  it('says the apply is still running instead of claiming there is none', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/runs/current': { state: 'running', dry_run: false, total_devices: 2 },
    })
    renderWithProviders(<ReportStep />, { path: '/report' })
    expect(await screen.findByText(/Uygulama sürüyor/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'İlerlemeyi gör' })).toHaveAttribute('href', '/apply')
    expect(screen.queryByText(/Henüz tamamlanmış/)).not.toBeInTheDocument()
  })
})
