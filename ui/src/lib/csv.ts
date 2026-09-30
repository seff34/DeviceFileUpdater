import type { Device } from './types'

function splitLine(line: string, sep: string): string[] {
  const out: string[] = []
  let cur = ''
  let quoted = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (quoted) {
      if (ch === '"' && line[i + 1] === '"') {
        cur += '"'
        i++
      } else if (ch === '"') quoted = false
      else cur += ch
    } else if (ch === '"' && cur === '') quoted = true
    else if (ch === sep) {
      out.push(cur)
      cur = ''
    } else cur += ch
  }
  out.push(cur)
  return out
}

/**
 * Parses pasted rows or a devices.csv file: tab, ';' or ',' separated,
 * optional header, optional BOM, password column optional.
 */
export function parseDeviceText(text: string): { devices: Device[]; errors: string[] } {
  const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/)
  const sample = lines.find((l) => l.trim() !== '') ?? ''
  const sep = sample.includes('\t') ? '\t' : sample.includes(';') && !sample.includes(',') ? ';' : ','
  const devices: Device[] = []
  const errors: string[] = []
  lines.forEach((raw, i) => {
    if (raw.trim() === '') return
    const f = splitLine(raw, sep)
    if (devices.length === 0 && errors.length === 0 && ['ip', 'host'].includes(f[0].trim().toLowerCase())) return
    const n = i + 1
    if (f.length > 3) return void errors.push(`Satır ${n}: en fazla 3 sütun olmalı (ip, kullanıcı, şifre).`)
    if (f.length < 2) return void errors.push(`Satır ${n}: en az IP ve kullanıcı adı olmalı.`)
    const host = f[0].trim()
    const username = f[1].trim()
    if (!host) return void errors.push(`Satır ${n}: IP gerekli.`)
    if (!username) return void errors.push(`Satır ${n}: kullanıcı adı gerekli.`)
    devices.push({ host, username, password: f[2] ?? '' })
  })
  return { devices, errors }
}
