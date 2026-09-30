import { describe, expect, it, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SettingsStep } from './SettingsStep'
import { json, mockApi, renderWithProviders, type Call } from '@/test/utils'
import { validateSettings } from '@/lib/validate'
import type { Settings } from '@/lib/types'

let saved: Settings
let calls: Call[]
const base = (): Settings => ({ parallel: 10, backup: true, post_command: '', post_command_policy: 'on_change', connect_timeout_sec: 10, command_timeout_sec: 30, strict_host_key: false })

beforeEach(() => {
  saved = base()
  calls = mockApi({
    '/api/workspace': { current: '/srv/ws', recent: [] },
    '/api/devices': { devices: [] },
    '/api/manifest': { entries: [] },
    'GET /api/settings': () => saved,
    'PUT /api/settings': ({ body }: Call) => (saved = body as Settings),
    '/api/runs/current': { state: 'idle' },
  })
})

const lastPut = () => calls.filter((c) => c.method === 'PUT').at(-1)?.body as Settings | undefined

describe('validateSettings', () => {
  it('mirrors Settings.Validate bounds', () => {
    expect(validateSettings(base())).toEqual({})
    expect(validateSettings({ ...base(), parallel: 200, connect_timeout_sec: 1, command_timeout_sec: 100000 })).toEqual({})
    expect(validateSettings({ ...base(), connect_timeout_sec: 2147483647 })).toEqual({})
    expect(Object.keys(validateSettings({ ...base(), connect_timeout_sec: 2147483648 }))).toEqual(['connect_timeout_sec'])
    expect(Object.keys(validateSettings({ ...base(), command_timeout_sec: 1e300 }))).toEqual(['command_timeout_sec'])
    expect(Object.keys(validateSettings({ ...base(), command_timeout_sec: Number.MAX_SAFE_INTEGER + 2 }))).toEqual(['command_timeout_sec'])
    expect(Object.keys(validateSettings({ ...base(), parallel: 201 }))).toEqual(['parallel'])
    expect(Object.keys(validateSettings({ ...base(), parallel: 0 }))).toEqual(['parallel'])
    expect(Object.keys(validateSettings({ ...base(), parallel: 1.5 }))).toEqual(['parallel'])
    expect(Object.keys(validateSettings({ ...base(), parallel: Number.NaN }))).toEqual(['parallel'])
    expect(Object.keys(validateSettings({ ...base(), connect_timeout_sec: 0 }))).toEqual(['connect_timeout_sec'])
    expect(Object.keys(validateSettings({ ...base(), command_timeout_sec: 0 }))).toEqual(['command_timeout_sec'])
    expect(Object.keys(validateSettings({ ...base(), post_command_policy: 'x' as Settings['post_command_policy'] }))).toEqual(['post_command_policy'])
  })
})

describe('SettingsStep', () => {
  it('renders no inputs or save indicator while settings are loading', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [] },
      '/api/manifest': { entries: [] },
      'GET /api/settings': () => new Promise<Response>(() => {}),
      '/api/runs/current': { state: 'idle' },
    })
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    expect(screen.queryByLabelText('Paralel cihaz sayısı')).not.toBeInTheDocument()
    expect(screen.queryByText('Kaydedildi')).not.toBeInTheDocument()
    expect(screen.getByText('Ayarlar yükleniyor.')).toBeInTheDocument()
  })
  it('shows an error and a neutral blocker when loading fails', async () => {
    mockApi({
      '/api/workspace': { current: '/srv/ws', recent: [] },
      '/api/devices': { devices: [] },
      '/api/manifest': { entries: [] },
      'GET /api/settings': () => json({ error: 'boom' }, 500),
      '/api/runs/current': { state: 'idle' },
    })
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    expect(await screen.findByText('boom')).toBeInTheDocument()
    expect(screen.getByText('Ayarlar yüklenemedi.')).toBeInTheDocument()
    expect(screen.queryByText('Kaydedildi')).not.toBeInTheDocument()
  })
  it('validates parallelism inline and saves valid values', async () => {
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    const par = await screen.findByLabelText('Paralel cihaz sayısı')
    await userEvent.clear(par)
    await userEvent.type(par, '0')
    const msg = await screen.findByText('1 ile 200 arasında bir tam sayı girin.')
    expect(par).toHaveAttribute('aria-invalid', 'true')
    expect(par.getAttribute('aria-describedby')).toContain(msg.id)
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(0)
    await userEvent.clear(par)
    await userEvent.type(par, '25')
    await waitFor(() => expect(lastPut()?.parallel).toBe(25), { timeout: 2000 })
    expect(par).toHaveAttribute('aria-invalid', 'false')
  })
  it('saves post-command, policy and advanced options', async () => {
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    await userEvent.type(await screen.findByLabelText('post-command'), 'systemctl restart app')
    await userEvent.click(screen.getByRole('button', { name: /Gelişmiş/ }))
    await userEvent.click(screen.getByRole('switch', { name: 'Host key doğrulaması' }))
    await waitFor(() => expect(lastPut()).toMatchObject({ post_command: 'systemctl restart app', strict_host_key: true }), { timeout: 2000 })
  })
  it('shows an error count on the collapsed Advanced trigger', async () => {
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    const trigger = await screen.findByRole('button', { name: /Gelişmiş/ })
    await userEvent.click(trigger)
    const t = await screen.findByLabelText('Bağlantı zaman aşımı (sn)')
    await userEvent.clear(t)
    expect(await screen.findByText('En az 1 saniye olan bir tam sayı girin.')).toBeInTheDocument()
    await userEvent.click(trigger)
    expect(screen.queryByLabelText('Bağlantı zaman aşımı (sn)')).not.toBeInTheDocument()
    expect(trigger).toHaveTextContent('Gelişmiş(1 hata)')
  })
  it('explains the policy for the selected value and when the command is empty', async () => {
    renderWithProviders(<SettingsStep />, { path: '/settings' })
    expect(await screen.findByText('Önce komut girin.')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('post-command'), 'x')
    expect(screen.getByText(/hiçbir dosya güncellenmediyse komut çalışmaz/)).toBeInTheDocument()
  })
})
