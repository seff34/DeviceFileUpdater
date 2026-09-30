import type { Device, ManifestEntry } from './types'

export type RowErrors = Partial<Record<string, string>> | null

/** Mirrors workspace.ValidateDevices. Returns one entry per row: null = valid. */
export function validateDevices(ds: Device[]): RowErrors[] {
  const seen = new Map<string, number>()
  return ds.map((d, i) => {
    const errs: Record<string, string> = {}
    const host = d.host.trim()
    if (!host) errs.host = 'IP gerekli.'
    else if (/\s/.test(host)) errs.host = 'IP boşluk içeremez.'
    else if (seen.has(host)) errs.host = `Bu IP ${seen.get(host)! + 1}. satırda da var.`
    if (!d.username.trim()) errs.username = 'Kullanıcı adı gerekli.'
    if (host && !seen.has(host)) seen.set(host, i)
    return Object.keys(errs).length ? errs : null
  })
}

const MODE_RE = /^[0-7]{3,4}$/

/** Mirrors workspace.ValidateEntries (row level). */
export function validateManifest(es: ManifestEntry[]): RowErrors[] {
  const seen = new Map<string, number>()
  return es.map((e, i) => {
    const errs: Record<string, string> = {}
    const remote = e.remote_path.trim()
    if (!e.local_path.trim()) errs.local_path = 'Dosya gerekli.'
    if (!remote.startsWith('/')) errs.remote_path = 'Hedef yol / ile başlamalı (mutlak yol).'
    // eslint-disable-next-line no-control-regex -- rejecting control characters is the point
    else if (/[\u0000-\u001f\u007f]/.test(remote)) errs.remote_path = 'Hedef yolda kontrol karakteri var.'
    else if (seen.has(remote)) errs.remote_path = `Bu hedef ${seen.get(remote)! + 1}. satırda da var.`
    if (e.mode.trim() && !MODE_RE.test(e.mode.trim())) errs.mode = 'İzin 0644 gibi sekizlik olmalı.'
    if (remote && !seen.has(remote)) seen.set(remote, i)
    return Object.keys(errs).length ? errs : null
  })
}
