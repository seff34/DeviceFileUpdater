import { runCounts, type RunView } from '@/lib/runState'

export function RunProgress({ view, total }: { view: RunView | null; total: number }) {
  const c = view ? runCounts(view) : { finished: 0, failed: 0, active: 0, total }
  const denom = Math.max(total, c.total, 1)
  const ratio = Math.min(c.finished / denom, 1)
  return (
    <div className="rounded-md border bg-card p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
        <p className="font-medium tabular-nums" role="status" aria-live="polite">
          {c.finished}/{denom} cihaz tamamlandı
        </p>
        <p className="tabular-nums text-muted-foreground">
          {c.active} işleniyor{c.failed > 0 && <span className="text-fail"> · {c.failed} başarısız</span>}
        </p>
      </div>
      <div className="mt-3 h-1.5 overflow-hidden rounded-md bg-muted" aria-hidden>
        <div
          className="h-full origin-left rounded-md bg-primary transition-transform duration-200 ease-[var(--ease-out-strong)] motion-reduce:transition-none"
          style={{ transform: `scaleX(${ratio})` }}
        />
      </div>
    </div>
  )
}
