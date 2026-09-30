import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { WorkspaceStep } from './WorkspaceStep'
import { json, mockApi, renderWithProviders } from '@/test/utils'

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
  it('clears the error Alert after a failed open is followed by a successful one', async () => {
    let current = ''
    let attempts = 0
    mockApi({
      'GET /api/workspace': () => ({ current, recent: ['/srv/bad', '/srv/good'] }),
      'POST /api/workspace': ({ body }: { body: { path: string } }) => {
        attempts++
        if (body.path === '/srv/bad') return json({ error: 'klasör bulunamadı' }, 400)
        current = body.path
        return { current, recent: [current] }
      },
      '/api/fs': { path: '/home/op', parent: '/home', roots: [], entries: [] },
    })
    renderWithProviders(<WorkspaceStep />, { path: '/workspace' })
    await userEvent.click(await screen.findByRole('button', { name: /\/srv\/bad/ }))
    expect(await screen.findByText('klasör bulunamadı')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /\/srv\/good/ }))
    expect(await screen.findByText('Açık çalışma alanı')).toBeInTheDocument()
    expect(attempts).toBe(2)
    expect(screen.queryByText('klasör bulunamadı')).not.toBeInTheDocument()
  })
})
