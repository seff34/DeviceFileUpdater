import { describe, expect, it } from 'vitest'
import { applyEvent, initialRunView, runCounts } from './runState'
import type { RunEvent, RunResult } from './types'

describe('run view reducer', () => {
  it('tracks stages, files and the authoritative result', () => {
    let v = initialRunView(['a', 'b'])
    expect(v.devices.a.stage).toBe('queued')
    const evs: RunEvent[] = [
      { type: 'device_state', host: 'a', stage: 'connecting', done: 0, total: 1 },
      { type: 'device_state', host: 'a', stage: 'syncing', done: 0, total: 1 },
      { type: 'file_result', host: 'a', done: 1, total: 1, file: { remote: '/x', status: 'CREATED', duration_ms: 5 } },
      { type: 'device_state', host: 'a', stage: 'done', done: 1, total: 1 },
      { type: 'device_state', host: 'b', stage: 'failed', done: 1, total: 1, error: 'connect: refused' },
    ]
    for (const e of evs) v = applyEvent(v, e)
    expect(v.devices.a.stage).toBe('done')
    expect(v.devices.a.files).toHaveLength(1)
    expect(v.devices.b.error).toBe('connect: refused')
    expect(runCounts(v)).toEqual({ total: 2, finished: 2, failed: 1, active: 0 })

    const result: RunResult = {
      id: 'r', started: '', finished: '', dry_run: false, parallel: 2, files: ['/x'],
      devices: [
        { host: 'a', duration_ms: 1, files: [{ remote: '/x', status: 'CREATED', duration_ms: 5 }] },
        { host: 'b', duration_ms: 1, error: 'connect: refused', files: [{ remote: '/x', status: 'FAILED', duration_ms: 0, error: 'device unreachable' }] },
      ],
    }
    v = applyEvent(v, { type: 'run_done', done: 0, total: 0, run: result, report_id: 'r' })
    expect(v.reportId).toBe('r')
    expect(v.devices.b.files[0].status).toBe('FAILED')
  })
  it('ignores events for unknown hosts safely by adding them', () => {
    const v = applyEvent(initialRunView([]), { type: 'device_state', host: 'z', stage: 'probing', done: 0, total: 3 })
    expect(v.order).toEqual(['z'])
  })
})
