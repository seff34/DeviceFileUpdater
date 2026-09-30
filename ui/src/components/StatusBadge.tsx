import { STATUS_META, TONE_CLASS } from '@/lib/status'
import type { Status } from '@/lib/types'
import { cn } from '@/lib/utils'

export function StatusBadge({ status, className }: { status: Status; className?: string }) {
  const m = STATUS_META[status]
  const Icon = m.icon
  return (
    <span className={cn('inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap', TONE_CLASS[m.tone], className)}>
      <Icon size={14} weight="bold" aria-hidden />
      {m.label}
    </span>
  )
}
