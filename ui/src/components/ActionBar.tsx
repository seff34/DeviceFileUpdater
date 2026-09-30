import { ArrowLeft, ArrowRight, WarningCircle } from '@phosphor-icons/react'
import { Button } from '@/components/ui/button'
import type { ReactNode } from 'react'

export interface ActionConfig {
  onBack?: () => void
  backLabel?: string
  primaryLabel?: string
  onPrimary?: () => void
  primaryDisabled?: boolean
  primaryBusy?: boolean
  blocker?: string | null
  extra?: ReactNode
}

export function ActionBar({ onBack, backLabel = 'Geri', primaryLabel = 'Devam', onPrimary, primaryDisabled, primaryBusy, blocker, extra }: ActionConfig) {
  return (
    <div className="sticky bottom-0 z-10 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="mx-auto flex max-w-6xl items-center gap-3 px-6 py-3">
        {onBack ? (
          <Button variant="outline" onClick={onBack}>
            <ArrowLeft aria-hidden /> {backLabel}
          </Button>
        ) : (
          <span />
        )}
        <div className="ml-auto flex items-center gap-3">
          {blocker && (
            <p className="flex items-center gap-1.5 text-sm text-muted-foreground" role="status">
              <WarningCircle size={16} className="text-warn" aria-hidden />
              {blocker}
            </p>
          )}
          {extra}
          {onPrimary && (
            <Button onClick={onPrimary} disabled={primaryDisabled || !!blocker || primaryBusy} aria-busy={primaryBusy}>
              {primaryLabel} <ArrowRight aria-hidden />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
