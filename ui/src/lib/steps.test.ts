import { describe, expect, it } from 'vitest'
import { blocker, canEnter, stepStates, type WizardFacts } from './steps'
import type { RunStatus } from './types'

const run = (p: Partial<RunStatus>): RunStatus => ({
  state: 'idle', dry_run: false, only_failed_from: '', total_devices: 0,
  report_id: '', preview_id: '', preview_fresh: false, error: '', ...p,
})

const ready: WizardFacts = {
  workspace: true, devices: 2, devicesValid: true, files: 1, filesValid: true,
  settingsValid: true, run: run({ state: 'done', dry_run: true, preview_id: 'p', preview_fresh: true, report_id: 'p' }),
  previewConfirmed: true,
}

describe('wizard gating', () => {
  it('blocks each step for the right reason', () => {
    expect(blocker('workspace', { ...ready, workspace: false })).toBe('Önce bir çalışma alanı açın.')
    expect(blocker('devices', { ...ready, devices: 0 })).toBe('En az bir cihaz ekleyin.')
    expect(blocker('files', { ...ready, filesValid: false })).toBe('Dosya satırlarındaki hataları düzeltin.')
    expect(blocker('preview', { ...ready, run: run({ preview_id: 'p', preview_fresh: false }) })).toBe('Önce güncel bir önizleme çalıştırın.')
    expect(blocker('preview', { ...ready, previewConfirmed: false })).toBe('Değişiklikleri onaylayın.')
    expect(blocker('preview', ready)).toBeNull()
  })
  it('only allows entering a step when all earlier steps are unblocked', () => {
    const noFiles = { ...ready, files: 0 }
    expect(canEnter('settings', noFiles)).toBe(false)
    expect(canEnter('files', noFiles)).toBe(true)
    expect(canEnter('apply', ready)).toBe(true)
    expect(canEnter('report', ready)).toBe(false)
  })
  it('locks every step except apply while a real run is running', () => {
    const running = { ...ready, run: run({ state: 'running', dry_run: false }) }
    expect(canEnter('devices', running)).toBe(false)
    expect(canEnter('apply', running)).toBe(true)
    expect(canEnter('report', running)).toBe(false)
  })
  it('opens the report after a real run finishes', () => {
    const applied = { ...ready, run: run({ state: 'done', dry_run: false, report_id: 'r1', preview_id: 'p', preview_fresh: true }) }
    expect(canEnter('report', applied)).toBe(true)
    expect(canEnter('devices', applied)).toBe(true)
  })
  it('computes step states', () => {
    const s = stepStates('files', { ...ready, files: 0 })
    expect(s.workspace).toBe('done')
    expect(s.files).toBe('current')
    expect(s.settings).toBe('blocked')
  })
})
