import { deviceFailed } from './status'
import type { FileResult, RunEvent, RunResult, Stage } from './types'

export interface DeviceView {
  host: string
  stage: Stage | 'queued'
  done: number
  total: number
  files: FileResult[]
  error?: string
}

export interface RunView {
  order: string[]
  devices: Record<string, DeviceView>
  result?: RunResult
  reportId?: string
}

export function initialRunView(hosts: string[]): RunView {
  const devices: Record<string, DeviceView> = {}
  hosts.forEach((h) => (devices[h] = { host: h, stage: 'queued', done: 0, total: 0, files: [] }))
  return { order: [...hosts], devices }
}

function upsert(v: RunView, host: string, patch: (d: DeviceView) => DeviceView): RunView {
  const cur = v.devices[host] ?? { host, stage: 'queued' as const, done: 0, total: 0, files: [] }
  return {
    ...v,
    order: v.order.includes(host) ? v.order : [...v.order, host],
    devices: { ...v.devices, [host]: patch(cur) },
  }
}

export function applyEvent(v: RunView, e: RunEvent): RunView {
  switch (e.type) {
    case 'device_state':
      if (!e.host || !e.stage) return v
      return upsert(v, e.host, (d) => ({
        ...d,
        stage: e.stage!,
        total: e.total,
        done: Math.max(d.done, e.done),
        error: e.error || d.error,
      }))
    case 'file_result':
      if (!e.host || !e.file) return v
      return upsert(v, e.host, (d) => ({ ...d, files: [...d.files, e.file!], done: e.done, total: e.total }))
    case 'run_done': {
      if (!e.run) return v
      let next: RunView = { ...v, result: e.run, reportId: e.report_id }
      for (const dr of e.run.devices) {
        next = upsert(next, dr.host, (d) => ({
          ...d,
          stage: deviceFailed(dr) ? 'failed' : 'done',
          files: dr.files,
          done: dr.files.length,
          total: dr.files.length,
          error: dr.error || d.error,
        }))
      }
      return next
    }
  }
}

export function runCounts(v: RunView) {
  const ds = v.order.map((h) => v.devices[h])
  const finished = ds.filter((d) => d.stage === 'done' || d.stage === 'failed').length
  return {
    total: ds.length,
    finished,
    failed: ds.filter((d) => d.stage === 'failed').length,
    active: ds.filter((d) => d.stage !== 'queued' && d.stage !== 'done' && d.stage !== 'failed').length,
  }
}
