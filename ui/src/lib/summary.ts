import { deviceFailed } from './status'
import type { DeviceResult, RunResult, Status } from './types'

export function tally(r: RunResult) {
  const t = {
    devices: r.devices.length,
    devCreate: 0, devUpdate: 0, filesCreate: 0, filesUpdate: 0,
    upToDate: 0, unreachable: 0, fileErrors: 0, changedDevices: 0,
    failedDevices: 0,
    byStatus: {} as Partial<Record<Status, number>>,
  }
  for (const d of r.devices) {
    let create = 0
    let update = 0
    for (const f of d.files) {
      t.byStatus[f.status] = (t.byStatus[f.status] ?? 0) + 1
      if (f.status === 'WOULD_CREATE' || f.status === 'CREATED') create++
      if (f.status === 'WOULD_UPDATE' || f.status === 'UPDATED') update++
    }
    const failed = deviceFailed(d)
    if (failed) t.failedDevices++
    if (d.error) t.unreachable++
    else if (failed) t.fileErrors++
    if (create) {
      t.devCreate++
      t.filesCreate += create
    }
    if (update) {
      t.devUpdate++
      t.filesUpdate += update
    }
    if (create || update) t.changedDevices++
    if (!failed && !create && !update) t.upToDate++
  }
  return t
}

export function previewSentence(r: RunResult): string {
  const t = tally(r)
  const change: string[] = []
  if (t.devCreate) change.push(`${t.devCreate} cihazda ${t.filesCreate} dosya oluşturulacak`)
  if (t.devUpdate) change.push(`${t.devUpdate} cihazda ${t.filesUpdate} dosya güncellenecek`)
  const rest: string[] = []
  if (t.upToDate) rest.push(`${t.upToDate} cihaz zaten güncel`)
  if (t.unreachable) rest.push(`${t.unreachable} cihaza ulaşılamadı`)
  if (t.fileErrors) rest.push(`${t.fileErrors} cihazda dosya hatası var`)
  const first = change.length ? change.join(', ') + '.' : 'Hiçbir cihazda değişiklik yok.'
  return rest.length ? `${first} ${rest.join(', ')}.` : first
}

export function resultSentence(r: RunResult): string {
  const t = tally(r)
  const ok = t.devices - t.failedDevices
  const head = t.failedDevices
    ? `${t.devices} cihazdan ${ok} tanesi başarılı, ${t.failedDevices} tanesi başarısız.`
    : `${t.devices} cihazın tamamı başarılı.`
  const parts: string[] = []
  const b = t.byStatus
  if (b.CREATED) parts.push(`${b.CREATED} dosya oluşturuldu`)
  if (b.UPDATED) parts.push(`${b.UPDATED} dosya güncellendi`)
  if (b.UNCHANGED) parts.push(`${b.UNCHANGED} dosya aynıydı`)
  if (b.FAILED) parts.push(`${b.FAILED} dosya başarısız`)
  return parts.length ? `${head} ${parts.join(', ')}.` : head
}

export type MatrixFilter = 'all' | 'changes' | 'failed'

const changes = (d: DeviceResult) =>
  d.files.some((f) => f.status === 'WOULD_CREATE' || f.status === 'WOULD_UPDATE' || f.status === 'CREATED' || f.status === 'UPDATED')

export function filterDevices(r: RunResult, filter: MatrixFilter) {
  return r.devices.filter((d) => (filter === 'all' ? true : filter === 'failed' ? deviceFailed(d) : changes(d)))
}

