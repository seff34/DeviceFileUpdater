import { markSessionLost } from './session'
import type {
  CheckResult, Device, FsListing, ManifestEntry, ManifestRow, ReportSummary,
  RunEvent, RunResult, RunStatus, Settings, UploadedFile, WorkspaceState,
} from './types'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function fail(res: Response): Promise<never> {
  let msg = `İstek başarısız (${res.status})`
  try {
    const j = await res.json()
    if (j && typeof j.error === 'string') msg = j.error
  } catch {
    /* non-JSON error body */
  }
  if (res.status === 401) markSessionLost()
  throw new ApiError(res.status, msg)
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin' }
  if (body instanceof FormData || typeof body === 'string') {
    init.body = body
  } else if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }
  const res = await fetch(path, init)
  if (!res.ok) return fail(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  workspace: () => request<WorkspaceState>('GET', '/api/workspace'),
  openWorkspace: (path: string, create: boolean) => request<WorkspaceState>('POST', '/api/workspace', { path, create }),
  fs: (path?: string) => request<FsListing>('GET', '/api/fs' + (path ? `?path=${encodeURIComponent(path)}` : '')),
  devices: () => request<{ devices: Device[] }>('GET', '/api/devices').then((r) => r.devices),
  saveDevices: (devices: Device[]) => request<{ devices: Device[] }>('PUT', '/api/devices', { devices }).then((r) => r.devices),
  manifest: () => request<{ entries: ManifestRow[] }>('GET', '/api/manifest').then((r) => r.entries),
  saveManifest: (entries: ManifestEntry[]) =>
    request<{ entries: ManifestRow[] }>('PUT', '/api/manifest', { entries }).then((r) => r.entries),
  uploadFiles: (files: File[]) => {
    const fd = new FormData()
    files.forEach((f) => fd.append('file', f, f.name))
    return request<{ files: UploadedFile[] }>('POST', '/api/files', fd).then((r) => r.files)
  },
  settings: () => request<Settings>('GET', '/api/settings'),
  saveSettings: (s: Settings) => request<Settings>('PUT', '/api/settings', s),
  runStatus: () => request<RunStatus>('GET', '/api/runs/current'),
  startRun: (req: { dry_run: boolean; preview_id?: string; only_failed_from?: string }) =>
    request<RunStatus>('POST', '/api/runs', req),
  cancelRun: () => request<void>('DELETE', '/api/runs/current'),
  reports: () => request<{ reports: ReportSummary[] }>('GET', '/api/reports').then((r) => r.reports),
  report: (id: string) => request<RunResult>('GET', `/api/reports/${encodeURIComponent(id)}`),
}

export const EXPORT_DEVICES_URL = '/api/devices/export'
export const reportHtmlUrl = (id: string, download = false) =>
  `/api/reports/${encodeURIComponent(id)}/html${download ? '?download=1' : ''}`

/** Reads newline-delimited JSON, calling onValue per complete line. */
export async function readNdjson(body: ReadableStream<Uint8Array>, onValue: (v: unknown) => void) {
  const reader = body.getReader()
  const dec = new TextDecoder()
  let buf = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buf += dec.decode(value, { stream: true })
    let i: number
    while ((i = buf.indexOf('\n')) >= 0) {
      const line = buf.slice(0, i).trim()
      buf = buf.slice(i + 1)
      if (line) onValue(JSON.parse(line))
    }
  }
  buf += dec.decode()
  if (buf.trim()) onValue(JSON.parse(buf))
}

/** Streams connection-test results; resolves when the server sends {"done":true}. */
export async function testConnection(hosts: string[], onResult: (r: CheckResult) => void, signal?: AbortSignal) {
  const res = await fetch('/api/test-connection', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ hosts }),
    signal,
  })
  if (!res.ok || !res.body) return fail(res)
  let done = false
  await readNdjson(res.body, (v) => {
    const o = v as CheckResult & { done?: boolean }
    if (o.done) done = true
    else onResult(o)
  })
  if (!done) throw new Error('Bağlantı testi yarıda kesildi, sonuçlar eksik olabilir.')
}

/**
 * Subscribes to the current run's SSE stream. The server replays from the
 * start on every (re)connection, so onReset fires before each replay.
 */
export function subscribeRun(h: { onReset: () => void; onEvent: (e: RunEvent) => void; onClosed?: () => void }) {
  const es = new EventSource('/api/runs/current/events')
  es.onopen = () => h.onReset()
  es.onmessage = (m) => {
    const e = JSON.parse(m.data) as RunEvent
    h.onEvent(e)
    if (e.type === 'run_done') {
      es.close()
      h.onClosed?.()
    }
  }
  es.onerror = () => {
    if (es.readyState === EventSource.CLOSED) h.onClosed?.()
  }
  return () => es.close()
}
