import { describe, expect, it, beforeEach } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DevicesStep } from './DevicesStep'
import { mockApi, renderWithProviders, type Call } from '@/test/utils'
import type { Device } from '@/lib/types'

let saved: Device[]
let calls: Call[]

beforeEach(() => {
  saved = [
    { host: '10.0.0.1', username: 'root', password: 'gizli1' },
    { host: '10.0.0.2', username: 'root', password: 'gizli2' },
  ]
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    'GET /api/devices': () => ({ devices: saved }),
    'PUT /api/devices': ({ body }: Call) => {
      saved = (body as { devices: Device[] }).devices
      return { devices: saved }
    },
    '/api/manifest': { entries: [] },
    '/api/settings': {},
    '/api/runs/current': { state: 'idle' },
    'POST /api/test-connection': () =>
      new Response(
        '{"host":"10.0.0.1","ok":true,"protocol":"ssh","upload_methods":["sftp","scp"],"hash_method":"sha256sum","tools":["base64","sha256sum"]}\n' +
          '{"host":"10.0.0.2","ok":false,"error":"Cihaza ulaşılamadı: connection refused"}\n{"done":true}\n',
      ),
  })
})

const puts = () => calls.filter((c) => c.method === 'PUT')

describe('DevicesStep', () => {
  it('masks passwords and reveals one on request', async () => {
    renderWithProviders(<DevicesStep />, { path: '/devices' })
    const pw = await screen.findByLabelText('Şifre, satır 1')
    expect(pw).toHaveAttribute('type', 'password')
    await userEvent.click(screen.getAllByRole('button', { name: 'Şifreyi göster' })[0])
    expect(pw).toHaveAttribute('type', 'text')
  })
  it('auto-saves a valid edit', async () => {
    renderWithProviders(<DevicesStep />, { path: '/devices' })
    const host = await screen.findByLabelText('IP, satır 2')
    await userEvent.clear(host)
    await userEvent.type(host, '10.0.0.9')
    await waitFor(() => expect(puts().length).toBeGreaterThan(0), { timeout: 2000 })
    expect((puts().at(-1)!.body as { devices: Device[] }).devices[1].host).toBe('10.0.0.9')
    expect(await screen.findByText('Kaydedildi')).toBeInTheDocument()
  })
  it('flags duplicates and does not save them', async () => {
    renderWithProviders(<DevicesStep />, { path: '/devices' })
    const host = await screen.findByLabelText('IP, satır 2')
    await userEvent.clear(host)
    await userEvent.type(host, '10.0.0.1')
    await userEvent.tab()
    expect(await screen.findByText('Bu IP 1. satırda da var.')).toBeInTheDocument()
    expect(screen.getByText('Cihaz listesindeki hataları düzeltin.')).toBeInTheDocument()
    await new Promise((r) => setTimeout(r, 600))
    expect(puts()).toHaveLength(0)
  })
  it('tests connections and removes unreachable devices after confirmation', async () => {
    renderWithProviders(<DevicesStep />, { path: '/devices' })
    await userEvent.click(await screen.findByRole('button', { name: 'Tümünü test et' }))
    expect(await screen.findByText('Cihaza ulaşılamadı: connection refused')).toBeInTheDocument()
    expect(screen.getByText(/sftp/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Ulaşılamayanları çıkar' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Çıkar' }))
    await waitFor(() => expect(puts().length).toBeGreaterThan(0), { timeout: 2000 })
    expect((puts().at(-1)!.body as { devices: Device[] }).devices.map((d) => d.host)).toEqual(['10.0.0.1'])
  })
  it('bulk-pastes new devices and skips known ones', async () => {
    renderWithProviders(<DevicesStep />, { path: '/devices' })
    await userEvent.click(await screen.findByRole('button', { name: 'Toplu ekle' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByLabelText('Cihaz satırları'))
    await userEvent.paste('10.0.0.1\troot\tx\n10.0.0.5\troot\ty')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Listeye ekle (1)' }))
    await waitFor(() => expect(puts().length).toBeGreaterThan(0), { timeout: 2000 })
    expect((puts().at(-1)!.body as { devices: Device[] }).devices.map((d) => d.host)).toEqual(['10.0.0.1', '10.0.0.2', '10.0.0.5'])
  })
})
