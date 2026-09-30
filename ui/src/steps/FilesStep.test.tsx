import { describe, expect, it, beforeEach } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FilesStep } from './FilesStep'
import { json, mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { ManifestRow } from '@/lib/types'

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
    fireEvent.dragOver(zone)
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
})
