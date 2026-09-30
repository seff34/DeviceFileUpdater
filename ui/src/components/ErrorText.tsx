import { explainError } from '@/lib/explain'
import { cn } from '@/lib/utils'

/**
 * A device or file error: the Turkish explanation first, the raw engine text
 * below it as the technical detail. Unknown errors show the raw text alone.
 */
export function ErrorText({ raw, className }: { raw: string; className?: string }) {
  const hint = explainError(raw)
  if (!hint) return <span className={cn('line-clamp-2 break-words text-fail', className)} title={raw}>{raw}</span>
  return (
    <span className={cn('block min-w-0', className)}>
      <span className="block text-fail">{hint}</span>
      {/* line-clamp, not truncate: nowrap text would widen table cells instead of shortening. */}
      <span className="line-clamp-1 break-all font-mono text-xs text-muted-foreground" title={raw}>{raw}</span>
    </span>
  )
}
