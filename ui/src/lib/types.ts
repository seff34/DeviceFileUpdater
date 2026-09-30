export type Status = 'UNCHANGED' | 'CREATED' | 'UPDATED' | 'WOULD_CREATE' | 'WOULD_UPDATE' | 'FAILED'
export type Stage = 'connecting' | 'probing' | 'syncing' | 'post_command' | 'done' | 'failed'

export interface Device { host: string; username: string; password: string }
export interface ManifestEntry { local_path: string; remote_path: string; mode: string }
export interface FileInfo { exists: boolean; size: number; sha256: string }
export interface ManifestRow extends ManifestEntry { file: FileInfo }
export interface UploadedFile { local_path: string; size: number; sha256: string; replaced: boolean }
export interface Settings {
  parallel: number
  backup: boolean
  post_command: string
  post_command_policy: 'on_change' | 'always' | 'never'
  connect_timeout_sec: number
  command_timeout_sec: number
  strict_host_key: boolean
}
export interface WorkspaceState { current: string; recent: string[] }
export interface FsEntry { name: string; path: string; is_workspace: boolean }
export interface FsListing { path: string; parent: string; entries: FsEntry[]; roots: string[] }
export interface CheckResult {
  host: string
  ok: boolean
  protocol?: string
  tools?: string[]
  upload_methods?: string[]
  hash_method?: string
  error?: string
}
export interface FileResult { remote: string; status: Status; method?: string; duration_ms: number; error?: string; note?: string }
export interface PostResult { command: string; exit_code: number; output?: string; error?: string }
export interface DeviceResult {
  host: string
  protocol?: string
  upload_method?: string
  hash_method?: string
  tools?: string[]
  duration_ms: number
  error?: string
  files: FileResult[]
  post?: PostResult
}
export interface RunResult { id: string; started: string; finished: string; dry_run: boolean; parallel: number; files: string[]; devices: DeviceResult[] }
export type RunState = 'idle' | 'running' | 'done'
export interface RunStatus {
  state: RunState
  dry_run: boolean
  only_failed_from: string
  total_devices: number
  report_id: string
  preview_id: string
  preview_fresh: boolean
  error: string
}
export interface RunEvent {
  type: 'device_state' | 'file_result' | 'run_done'
  host?: string
  stage?: Stage
  done: number
  total: number
  file?: FileResult
  error?: string
  run?: RunResult
  report_id?: string
}
export interface ReportSummary {
  id: string
  started: string
  finished: string
  dry_run: boolean
  devices: number
  devices_failed: number
  by_status: Partial<Record<Status, number>>
}
