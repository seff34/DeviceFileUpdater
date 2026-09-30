import { describe, expect, it, beforeEach, vi } from 'vitest'
import { toast } from 'sonner'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FilesStep } from './FilesStep'
import { json, mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { ManifestRow } from '@/lib/types'

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const info = (size: number) => ({ exists: true, size, sha256: 'ab'.repeat(32) })
let rows: ManifestRow[]
let calls: Call[]
const puts = () => calls.filter((c) => c.method === 'PUT')

beforeEach(() => {
  rows = [
    { local_path: 'files/app.conf', remote_path: '/etc/app/app.conf', mode: '0644', file: info(2048) },
    { local_path: 'files/gone.bin', remote_path: '/opt/gone.bin', mode: '', file: { exists: false, size: 0, sha256: '' } },
  ]
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    '/api/devices': { devices: [] },
    'GET /api/manifest': () => ({ entries: rows }),
    'PUT /api/manifest': ({ body }: Call) => {
      const es = (body as { entries: ManifestRow[] }).entries
      rows = es.map((e) => ({ ...e, file: info(5) }))
      return { entries: rows }
    },
    'POST /api/files': { files: [{ local_path: 'files/start.sh', size: 5, sha256: 'cd'.repeat(32), replaced: false }] },
    '/api/settings': {},
    '/api/runs/current': { state: 'idle' },
  })
})

describe('FilesStep', () => {
  it('shows file details and flags missing files', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    expect(await screen.findByText('app.conf')).toBeInTheDocument()
    expect(screen.getByText(/2 KB · sha256 abababab/)).toBeInTheDocument()
    expect(screen.getByText(/Dosya bulunamadı/)).toBeInTheDocument()
    expect(screen.getByText('Dosya satırlarındaki hataları düzeltin.')).toBeInTheDocument()
  })
  it('uploads, completes a folder path with the file name, and saves', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    await userEvent.click(screen.getByRole('button', { name: 'Satır 2 sil' }))
    const file = new File(['echo'], 'start.sh', { type: 'text/x-sh' })
    await userEvent.upload(screen.getByLabelText('Dosya yükle'), file)
    const remote = await screen.findByLabelText('Hedef yol, satır 2')
    await waitFor(() => expect(remote).toHaveFocus())
    await userEvent.type(remote, '/usr/local/bin/')
    await userEvent.tab()
    expect(remote).toHaveValue('/usr/local/bin/start.sh')
    await waitFor(() => expect(puts().length).toBeGreaterThan(0), { timeout: 2000 })
    expect(puts().at(-1)!.body).toEqual({
      entries: [
        { local_path: 'files/app.conf', remote_path: '/etc/app/app.conf', mode: '0644' },
        { local_path: 'files/start.sh', remote_path: '/usr/local/bin/start.sh', mode: '' },
      ],
    })
  })
  it('uploads files dropped on the drop zone', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    const zone = screen.getByText('Dosyaları buraya bırakın').closest('div[data-dropzone]')!
    fireEvent.dragOver(zone, { dataTransfer: { types: ['Files'] } })
    expect(screen.getByText('Yüklemek için bırakın')).toBeInTheDocument()
    fireEvent.drop(zone, { dataTransfer: { files: [new File(['echo'], 'start.sh')] } })
    expect(await screen.findByLabelText('Hedef yol, satır 3')).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'POST')!.body).toEqual({ file: 'start.sh' })
    expect(screen.queryByText('Yüklemek için bırakın')).not.toBeInTheDocument()
  })
  it('opens the file picker from a focusable button', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    const input = screen.getByLabelText('Dosya yükle') as HTMLInputElement
    let opened = 0
    input.addEventListener('click', () => opened++)
    const btn = screen.getByRole('button', { name: 'Dosya seç' })
    btn.focus()
    await userEvent.keyboard('{Enter}')
    expect(opened).toBe(1)
  })
  it('rejects relative targets and bad modes', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    const remote = await screen.findByLabelText('Hedef yol, satır 1')
    await userEvent.clear(remote)
    await userEvent.type(remote, 'etc/app.conf')
    const mode = screen.getByLabelText('İzin, satır 1')
    await userEvent.clear(mode)
    await userEvent.type(mode, '999')
    expect(await screen.findByText('Hedef yol / ile başlamalı (mutlak yol).')).toBeInTheDocument()
    expect(screen.getByText('İzin 0644 gibi sekizlik olmalı.')).toBeInTheDocument()
  })
  it('keeps upload disabled while loading and after a load error', async () => {
    calls = mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [] },
      'GET /api/manifest': () => json({ error: 'manifest.csv okunamadı' }, 500),
      '/api/settings': {},
      '/api/runs/current': { state: 'idle' },
    })
    renderWithProviders(<FilesStep />, { path: '/files' })
    expect(screen.getByRole('button', { name: 'Dosya seç' })).toBeDisabled()
    expect(screen.getByLabelText('Dosya yükle')).toBeDisabled()
    expect(await screen.findByText('manifest.csv okunamadı')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dosya seç' })).toBeDisabled()
    expect(screen.getByLabelText('Dosya yükle')).toBeDisabled()
    const zone = screen.getByText('Dosyaları buraya bırakın').closest('div[data-dropzone]')!
    fireEvent.drop(zone, { dataTransfer: { files: [new File(['x'], 'a.txt')] } })
    expect(screen.queryByText('Kaydedildi')).not.toBeInTheDocument()
    expect(calls.filter((c) => c.method !== 'GET')).toHaveLength(0)
  })
  it('keeps an edit and a following upload when the upload lands during a pending auto-save', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    await userEvent.click(screen.getByRole('button', { name: 'Satır 2 sil' }))
    const remote = screen.getByLabelText('Hedef yol, satır 1')
    await userEvent.clear(remote)
    await userEvent.type(remote, '/etc/yeni.conf')
    // the save for this edit is still pending (400 ms debounce) when the upload runs
    await userEvent.upload(screen.getByLabelText('Dosya yükle'), new File(['echo'], 'start.sh'))
    await screen.findByLabelText('Hedef yol, satır 2')
    await userEvent.type(screen.getByLabelText('Hedef yol, satır 2'), '/bin/start.sh')
    await waitFor(() => expect(puts().length).toBeGreaterThan(0), { timeout: 2000 })
    expect(puts().at(-1)!.body).toEqual({
      entries: [
        { local_path: 'files/app.conf', remote_path: '/etc/yeni.conf', mode: '0644' },
        { local_path: 'files/start.sh', remote_path: '/bin/start.sh', mode: '' },
      ],
    })
  })
  it('keeps target and mode when a same-name upload replaces a file', async () => {
    calls = mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [] },
      'GET /api/manifest': () => ({ entries: rows }),
      'POST /api/files': { files: [{ local_path: 'files/app.conf', size: 9216, sha256: 'ef'.repeat(32), replaced: true }] },
      '/api/settings': {},
      '/api/runs/current': { state: 'idle' },
    })
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    await userEvent.upload(screen.getByLabelText('Dosya yükle'), new File(['x'], 'app.conf'))
    expect(await screen.findByText(/9 KB · sha256 efefefef/)).toBeInTheDocument()
    expect(screen.getByLabelText('Hedef yol, satır 1')).toHaveValue('/etc/app/app.conf')
    expect(screen.getByLabelText('İzin, satır 1')).toHaveValue('0644')
    expect(screen.getAllByLabelText(/Hedef yol, satır/)).toHaveLength(2)
    expect(toast.success).toHaveBeenCalledWith('1 dosya yüklendi, 1 tanesi eskisinin yerine geçti.')
  })
  it('warns and refetches the manifest when an upload fails', async () => {
    calls = mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [] },
      'GET /api/manifest': () => ({ entries: rows }),
      'POST /api/files': () => json({ error: 'disk dolu' }, 500),
      '/api/settings': {},
      '/api/runs/current': { state: 'idle' },
    })
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    const gets = () => calls.filter((c) => c.method === 'GET' && c.url === '/api/manifest').length
    const before = gets()
    await userEvent.upload(screen.getByLabelText('Dosya yükle'), new File(['x'], 'a.bin'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Bazı dosyalar sunucuya yazılmış olabilir. disk dolu'))
    await waitFor(() => expect(gets()).toBeGreaterThan(before))
  })
  it('refuses an oversize upload without sending a request', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    const big = new File(['x'], 'dev.img')
    Object.defineProperty(big, 'size', { value: 5 * 1024 ** 3 })
    await userEvent.upload(screen.getByLabelText('Dosya yükle'), big)
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('dev.img'))
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })
  it('flags a duplicate target inline and marks the field invalid', async () => {
    rows = [
      { local_path: 'files/a.conf', remote_path: '/etc/x', mode: '', file: info(1) },
      { local_path: 'files/b.conf', remote_path: '/etc/x', mode: '', file: info(1) },
    ]
    renderWithProviders(<FilesStep />, { path: '/files' })
    const second = await screen.findByLabelText('Hedef yol, satır 2')
    expect(await screen.findByText('Bu hedef 1. satırda da var.')).toBeInTheDocument()
    expect(second).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText('Hedef yol, satır 1')).toHaveAttribute('aria-invalid', 'false')
  })
  it('gives a neutral blocker while loading', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    expect(screen.queryByText('En az bir dosya ekleyin.')).not.toBeInTheDocument()
    expect(screen.getByText('Dosya listesi yükleniyor.')).toBeInTheDocument()
    await screen.findByText('app.conf')
  })
  it('ignores non-file drags', async () => {
    renderWithProviders(<FilesStep />, { path: '/files' })
    await screen.findByText('app.conf')
    const zone = screen.getByText('Dosyaları buraya bırakın').closest('div[data-dropzone]')!
    fireEvent.dragOver(zone, { dataTransfer: { types: ['text/plain'] } })
    expect(screen.queryByText('Yüklemek için bırakın')).not.toBeInTheDocument()
    fireEvent.dragOver(zone, { dataTransfer: { types: ['Files'] } })
    expect(screen.getByText('Yüklemek için bırakın')).toBeInTheDocument()
  })
})
