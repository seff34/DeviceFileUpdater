const num = (v: number, digits: number) => v.toLocaleString('tr-TR', { maximumFractionDigits: digits })

export function formatBytes(bytes: number): string {
  const n = Math.max(0, bytes)
  if (n < 1024) return `${Math.round(n)} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  const digits = (v: number) => (v < 10 ? 1 : 0)
  let v = n / 1024
  let i = 0
  // Roll over when the displayed value would read 1024 (e.g. 1048575 B is "1 MB").
  while (i < units.length - 1 && Number(v.toFixed(digits(v))) >= 1024) {
    v /= 1024
    i++
  }
  return `${num(v, digits(v))} ${units[i]}`
}

export function formatDuration(msIn: number): string {
  const ms = Math.max(0, msIn)
  if (Math.round(ms) < 1000) return `${Math.round(ms)} ms`
  const s = ms / 1000
  if (Number(s.toFixed(1)) < 60) return `${num(s, 1)} sn`
  const total = Math.round(s)
  const m = Math.floor(total / 60)
  const rest = total % 60
  return rest === 0 ? `${m} dk` : `${m} dk ${rest} sn`
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('tr-TR', { dateStyle: 'medium', timeStyle: 'short' })
}

export const baseName = (p: string) => p.split(/[\\/]/).pop() || p
