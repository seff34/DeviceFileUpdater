import { describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FolderBrowser } from './FolderBrowser'
import { mockApi, renderWithProviders } from '@/test/utils'

const listing = (path: string, parent: string, names: [string, boolean][]) => ({
  path, parent, roots: [],
  entries: names.map(([name, ws]) => ({ name, path: `${path}/${name}`, is_workspace: ws })),
})

describe('FolderBrowser', () => {
  it('walks into folders and opens the chosen one', async () => {
    mockApi({
      '/api/workspace': { current: '', recent: [] },
      '/api/fs': ({ url }: { url: string }) =>
        url.includes('path=')
          ? listing('/home/op/saha', '/home/op', [['hat-3', true]])
          : listing('/home/op', '/home', [['saha', false], ['belgeler', false]]),
    })
    const onOpen = vi.fn()
    renderWithProviders(<FolderBrowser onOpen={onOpen} busy={false} />)
    await userEvent.click(await screen.findByRole('button', { name: /saha/ }))
    expect(await screen.findByRole('button', { name: /hat-3/ })).toBeInTheDocument()
    expect(screen.getByText('Çalışma alanı')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Bu klasörü kullan' }))
    expect(onOpen).toHaveBeenCalledWith('/home/op/saha', true)
  })
  it('creates a named subfolder', async () => {
    mockApi({ '/api/workspace': { current: '', recent: [] }, '/api/fs': listing('/home/op', '/home', []) })
    const onOpen = vi.fn()
    renderWithProviders(<FolderBrowser onOpen={onOpen} busy={false} />)
    await userEvent.type(await screen.findByLabelText('Yeni klasör adı'), 'hat-4')
    await userEvent.click(screen.getByRole('button', { name: 'Oluştur ve kullan' }))
    expect(onOpen).toHaveBeenCalledWith('/home/op/hat-4', true)
  })
  it('rejects folder names with separators', async () => {
    mockApi({ '/api/workspace': { current: '', recent: [] }, '/api/fs': listing('/home/op', '/home', []) })
    renderWithProviders(<FolderBrowser onOpen={vi.fn()} busy={false} />)
    await userEvent.type(await screen.findByLabelText('Yeni klasör adı'), 'a/b')
    expect(screen.getByText('Klasör adı / veya \\ içeremez.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Oluştur ve kullan' })).toBeDisabled()
  })
  it('shows the new path in the address field after navigating in', async () => {
    mockApi({
      '/api/fs': ({ url }: { url: string }) =>
        url.includes('path=') ? listing('/home/op/saha', '/home/op', []) : listing('/home/op', '/home', [['saha', false]]),
    })
    renderWithProviders(<FolderBrowser onOpen={vi.fn()} busy={false} />)
    await waitFor(() => expect(screen.getByLabelText('Klasör yolu')).toHaveValue('/home/op'))
    await userEvent.click(await screen.findByRole('button', { name: /saha/ }))
    await waitFor(() => expect(screen.getByLabelText('Klasör yolu')).toHaveValue('/home/op/saha'))
  })
  it('goes to the parent folder with Üst klasör', async () => {
    mockApi({
      '/api/fs': ({ url }: { url: string }) =>
        url.includes('path=%2Fhome%2Fop%2Fsaha')
          ? listing('/home/op/saha', '/home/op', [])
          : listing('/home/op', '/home', [['saha', false]]),
    })
    renderWithProviders(<FolderBrowser onOpen={vi.fn()} busy={false} />)
    await userEvent.click(await screen.findByRole('button', { name: /saha/ }))
    await waitFor(() => expect(screen.getByLabelText('Klasör yolu')).toHaveValue('/home/op/saha'))
    await userEvent.click(screen.getByRole('button', { name: 'Üst klasör' }))
    await waitFor(() => expect(screen.getByLabelText('Klasör yolu')).toHaveValue('/home/op'))
  })
})
