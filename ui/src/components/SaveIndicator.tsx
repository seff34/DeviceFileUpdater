import { CheckCircle, CircleNotch, WarningCircle, XCircle, type Icon } from '@phosphor-icons/react'
import type { SaveState } from '@/lib/useDraft'
import { cn } from '@/lib/utils'

const META: Record<SaveState, [Icon, string, string]> = {
  saved: [CheckCircle, 'Kaydedildi', 'text-muted-foreground'],
  pending: [CircleNotch, 'Kaydediliyor', 'text-muted-foreground'],
  saving: [CircleNotch, 'Kaydediliyor', 'text-muted-foreground'],
  invalid: [WarningCircle, 'Hatalar düzeltilince kaydedilecek', 'text-warn'],
  error: [XCircle, 'Kaydedilemedi', 'text-fail'],
}

export function SaveIndicator({ state, error }: { state: SaveState; error?: string | null }) {
  const [Icon, label, tone] = META[state]
  const spinning = state === 'pending' || state === 'saving'
  return (
    <p role="status" aria-live="polite" className={cn('flex items-center gap-1.5 text-sm', tone)} title={state === 'error' ? error ?? undefined : undefined}>
      <Icon size={16} className={cn(spinning && 'animate-spin')} aria-hidden />
      {label}
    </p>
  )
}
