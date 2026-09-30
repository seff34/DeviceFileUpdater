import { ArrowsClockwise, Equals, PlusCircle, XCircle, type Icon } from '@phosphor-icons/react'
import type { DeviceResult, Stage, Status } from './types'

export type Tone = 'ok' | 'warn' | 'fail' | 'same'

export const STATUS_META: Record<Status, { label: string; icon: Icon; tone: Tone }> = {
  CREATED: { label: 'Oluşturuldu', icon: PlusCircle, tone: 'ok' },
  UPDATED: { label: 'Güncellendi', icon: ArrowsClockwise, tone: 'ok' },
  UNCHANGED: { label: 'Aynı', icon: Equals, tone: 'same' },
  WOULD_CREATE: { label: 'Oluşturulacak', icon: PlusCircle, tone: 'warn' },
  WOULD_UPDATE: { label: 'Güncellenecek', icon: ArrowsClockwise, tone: 'warn' },
  FAILED: { label: 'Başarısız', icon: XCircle, tone: 'fail' },
}
export const statusMeta = (s: Status) => STATUS_META[s]

export const TONE_CLASS: Record<Tone, string> = {
  ok: 'text-ok bg-ok/10 border-ok/25',
  warn: 'text-warn bg-warn/10 border-warn/25',
  fail: 'text-fail bg-fail/10 border-fail/25',
  same: 'text-same bg-same/10 border-same/20',
}

export const STAGE_LABEL: Record<Stage | 'queued', string> = {
  queued: 'Sırada',
  connecting: 'Bağlanıyor',
  probing: 'Araçlar tespit ediliyor',
  syncing: 'Dosyalar işleniyor',
  post_command: 'post-command çalışıyor',
  done: 'Tamamlandı',
  failed: 'Başarısız',
}

/** Mirrors model.DeviceResult.Failed on the Go side. */
export function deviceFailed(d: DeviceResult): boolean {
  if (d.error) return true
  if (d.post && (d.post.exit_code !== 0 || d.post.error)) return true
  return d.files.some((f) => f.status === 'FAILED')
}
