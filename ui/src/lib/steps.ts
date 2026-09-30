import type { RunStatus } from './types'

export const STEPS = [
  { id: 'workspace', path: '/workspace', title: 'Çalışma alanı', purpose: 'Cihaz listesi, dosyalar ve raporların tutulacağı klasörü seçin.' },
  { id: 'devices', path: '/devices', title: 'Cihazlar', purpose: 'Güncellenecek cihazları ekleyin ve bağlantıyı test edin.' },
  { id: 'files', path: '/files', title: 'Dosyalar', purpose: 'Gönderilecek dosyaları ve cihazdaki hedef yollarını belirleyin.' },
  { id: 'settings', path: '/settings', title: 'Ayarlar', purpose: 'Paralellik, yedekleme ve post-command ayarlarını yapın.' },
  { id: 'preview', path: '/preview', title: 'Önizleme', purpose: 'Dry-run ile hangi cihazda neyin değişeceğini görün ve onaylayın.' },
  { id: 'apply', path: '/apply', title: 'Uygula', purpose: 'Dosyalar cihazlara gönderiliyor. Her cihazı canlı izleyin.' },
  { id: 'report', path: '/report', title: 'Rapor', purpose: 'Sonuçları inceleyin, başarısız cihazları tekrar deneyin.' },
] as const

export type StepId = (typeof STEPS)[number]['id']
export type StepState = 'done' | 'current' | 'reachable' | 'blocked'

export interface WizardFacts {
  workspace: boolean
  devices: number
  devicesValid: boolean
  files: number
  filesValid: boolean
  settingsValid: boolean
  run: RunStatus | null
  previewConfirmed: boolean
}

const realRun = (f: WizardFacts) => !!f.run && !f.run.dry_run && f.run.state !== 'idle'

/** Why the operator cannot move forward from `step`, or null when they can. */
export function blocker(step: StepId, f: WizardFacts): string | null {
  switch (step) {
    case 'workspace':
      return f.workspace ? null : 'Önce bir çalışma alanı açın.'
    case 'devices':
      if (f.devices === 0) return 'En az bir cihaz ekleyin.'
      return f.devicesValid ? null : 'Cihaz listesindeki hataları düzeltin.'
    case 'files':
      if (f.files === 0) return 'En az bir dosya ekleyin.'
      return f.filesValid ? null : 'Dosya satırlarındaki hataları düzeltin.'
    case 'settings':
      return f.settingsValid ? null : 'Ayarlardaki hataları düzeltin.'
    case 'preview':
      if (f.run?.state === 'running' && f.run.dry_run) return 'Önizleme sürüyor.'
      if (!f.run?.preview_id || !f.run.preview_fresh) return 'Önce güncel bir önizleme çalıştırın.'
      return f.previewConfirmed ? null : 'Değişiklikleri onaylayın.'
    case 'apply':
      if (f.run?.state === 'running') return 'Uygulama sürüyor.'
      return realRun(f) && f.run!.state === 'done' ? null : 'Uygulama henüz tamamlanmadı.'
    case 'report':
      return null
  }
}

export function canEnter(step: StepId, f: WizardFacts): boolean {
  const running = f.run?.state === 'running' && !f.run.dry_run
  if (running) return step === 'apply'
  if (step === 'report') return realRun(f) && f.run!.state === 'done'
  if (step === 'apply' && realRun(f)) return true
  const idx = STEPS.findIndex((s) => s.id === step)
  return STEPS.slice(0, idx).every((s) => blocker(s.id, f) === null)
}

export function stepStates(current: StepId, f: WizardFacts): Record<StepId, StepState> {
  const out = {} as Record<StepId, StepState>
  const ci = STEPS.findIndex((s) => s.id === current)
  STEPS.forEach((s, i) => {
    if (s.id === current) out[s.id] = 'current'
    else if (!canEnter(s.id, f)) out[s.id] = 'blocked'
    else if (i < ci && blocker(s.id, f) === null) out[s.id] = 'done'
    else out[s.id] = 'reachable'
  })
  return out
}

export const stepById = (id: StepId) => STEPS.find((s) => s.id === id)!
export const stepByPath = (path: string) => STEPS.find((s) => s.path === path)
