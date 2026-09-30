import { describe, expect, it, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { WorkspaceStep } from './WorkspaceStep'
import { mockApi, renderWithProviders } from '@/test/utils'

beforeEach(() => vi.restoreAllMocks())

describe('WorkspaceStep', () => {
  it('opens a recent workspace without creating anything', async () => {
    let current = ''
    const calls = mockApi({
      'GET /api/workspace': () => ({ current, recent: ['/srv/hat-1', '/srv/hat-2'] }),
      'POST /api/workspace': ({ body }: { body: { path: string } }) => {
        current = body.path
        return { current, recent: [current] }
      },
      '/api/fs': { path: '/home/op', parent: '/home', roots: [], entries: [] },
    })
    renderWithProviders(<WorkspaceStep />, { path: '/workspace' })
    await userEvent.click(await screen.findByRole('button', { name: /\/srv\/hat-2/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && (c.body as { path: string }).path === '/srv/hat-2')).toBe(true))
    const post = calls.find((c) => c.method === 'POST')!
    expect(post.body).toEqual({ path: '/srv/hat-2', create: false })
    expect(await screen.findByText('Açık çalışma alanı')).toBeInTheDocument()
  })
  it('blocks Devam until a workspace is open', async () => {
    mockApi({
      '/api/workspace': { current: '', recent: [] },
      '/api/fs': { path: '/home/op', parent: '/home', roots: [], entries: [] },
    })
    renderWithProviders(<WorkspaceStep />, { path: '/workspace' })
    expect(await screen.findByText('Önce bir çalışma alanı açın.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Devam/ })).toBeDisabled()
  })
})
