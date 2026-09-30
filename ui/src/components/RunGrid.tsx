import { CheckCircle, Circle, CircleNotch, XCircle } from '@phosphor-icons/react'
import { memo, useMemo } from 'react'
import { STAGE_LABEL, STATUS_META } from '@/lib/status'
import type { DeviceView, RunView } from '@/lib/runState'
import type { FileResult, Status } from '@/lib/types'
import { cn } from '@/lib/utils'

export type GridFilter = 'all' | 'failed'

function StageCell({ stage }: { stage: DeviceView['stage'] }) {
  const busy = stage !== 'queued' && stage !== 'done' && stage !== 'failed'
  const Icon = stage === 'done' ? CheckCircle : stage === 'failed' ? XCircle : busy ? CircleNotch : Circle
  return (
    // Keyed by stage so each change remounts the cell and replays the 150ms crossfade.
    <span
      key={stage}
      className={cn(
        'stage-in inline-flex items-center gap-1.5',
        stage === 'done' && 'text-ok',
        stage === 'failed' && 'text-fail',
        stage === 'queued' && 'text-muted-foreground',
      )}
    >
      <Icon size={16} weight={busy || stage === 'queued' ? 'regular' : 'bold'} className={cn(busy && 'text-primary')} aria-hidden />
      {STAGE_LABEL[stage]}
    </span>
  )
}

function fileSummary(files: FileResult[]): string {
  const counts = new Map<Status, number>()
  files.forEach((f) => counts.set(f.status, (counts.get(f.status) ?? 0) + 1))
  return [...counts].map(([s, n]) => `${n} ${STATUS_META[s].label.toLocaleLowerCase('tr-TR')}`).join(' · ')
}

function detail(d: DeviceView): { text: string; tone?: string } {
  if (d.error === 'cancelled') return { text: 'İptal edildi', tone: 'text-fail' }
  if (d.error) return { text: d.error, tone: 'text-fail' }
  const failed = d.files.find((f) => f.status === 'FAILED')
  if (failed) return { text: `${failed.remote}: ${failed.error ?? 'başarısız'}`, tone: 'text-fail' }
  if (d.files.length) return { text: fileSummary(d.files) }
  return { text: '' }
}

// applyEvent keeps the identity of untouched devices, so memo skips every row an event did not change.
const RunRow = memo(function RunRow({ d }: { d: DeviceView }) {
  const det = detail(d)
  const ratio = d.total ? Math.min(d.done / d.total, 1) : 0
  return (
    <tr className="h-11">
      <th scope="row" className="truncate border-b px-3 text-left font-mono font-normal" title={d.host}>{d.host}</th>
      <td className="border-b px-3"><StageCell stage={d.stage} /></td>
      <td className="border-b px-3">
        <div className="flex items-center gap-2">
          <span className="w-10 font-mono text-xs tabular-nums">{d.total ? `${d.done}/${d.total}` : '-'}</span>
          <div className="h-1 flex-1 overflow-hidden rounded-md bg-muted" aria-hidden>
            <div
              className={cn('h-full origin-left rounded-md transition-transform duration-200 ease-[var(--ease-out-strong)] motion-reduce:transition-none', d.stage === 'failed' ? 'bg-fail' : 'bg-primary')}
              style={{ transform: `scaleX(${ratio})` }}
            />
          </div>
        </div>
      </td>
      <td className={cn('truncate border-b px-3', det.tone ?? 'text-muted-foreground')} title={det.text || undefined}>{det.text}</td>
    </tr>
  )
})

export function RunGrid({ view, filter }: { view: RunView; filter: GridFilter }) {
  const rows = useMemo(
    () => view.order.map((h) => view.devices[h]).filter((d) => filter === 'all' || d.stage === 'failed'),
    [view, filter],
  )
  return (
    <div className="max-h-[62vh] overflow-auto rounded-md border bg-card">
      <table className="w-full table-fixed border-separate border-spacing-0 text-sm">
        <colgroup>
          <col className="w-44" />
          <col className="w-56" />
          <col className="w-36" />
          <col />
        </colgroup>
        <thead className="sticky top-0 z-10 bg-card">
          <tr className="text-left text-muted-foreground">
            <th scope="col" className="border-b px-3 py-2 font-medium">Cihaz</th>
            <th scope="col" className="border-b px-3 py-2 font-medium">Aşama</th>
            <th scope="col" className="border-b px-3 py-2 font-medium">Dosyalar</th>
            <th scope="col" className="border-b px-3 py-2 font-medium">Ayrıntı</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((d) => <RunRow key={d.host} d={d} />)}
        </tbody>
      </table>
      {rows.length === 0 && (
        <p className="p-6 text-sm text-muted-foreground">{filter === 'failed' ? 'Başarısız cihaz yok.' : 'Gösterilecek cihaz yok.'}</p>
      )}
    </div>
  )
}
