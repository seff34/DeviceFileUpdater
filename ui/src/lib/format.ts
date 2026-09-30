const num = (v: number, digits: number) => v.toLocaleString('tr-TR', { maximumFractionDigits: digits })

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${num(v, v < 10 ? 1 : 0)} ${units[i]}`
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  const s = ms / 1000
  if (s < 60) return `${num(s, 1)} sn`
  return `${Math.floor(s / 60)} dk ${Math.round(s % 60)} sn`
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('tr-TR', { dateStyle: 'medium', timeStyle: 'short' })
}

export const baseName = (p: string) => p.split(/[\\/]/).pop() || p
