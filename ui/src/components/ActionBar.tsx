import { ArrowLeft, ArrowRight, WarningCircle } from '@phosphor-icons/react'
import { Button } from '@/components/ui/button'
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface ActionConfig {
  onBack?: () => void
  backLabel?: string
  primaryLabel?: string
  onPrimary?: () => void
  primaryDisabled?: boolean
  primaryBusy?: boolean
  /** outline when the page already has its own primary action */
  primaryVariant?: 'default' | 'outline'
  blocker?: string | null
  extra?: ReactNode
}

export function ActionBar({ onBack, backLabel = 'Geri', primaryLabel = 'Devam', onPrimary, primaryDisabled, primaryBusy, primaryVariant, blocker, extra }: ActionConfig) {
  return (
    <div className="sticky bottom-0 z-10 border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-3 gap-y-2 px-6 py-3">
        {onBack ? (
          <Button variant="outline" onClick={onBack}>
            <ArrowLeft aria-hidden /> {backLabel}
          </Button>
        ) : (
          <span />
        )}
        {/* On a phone the reason sits on its own line above the buttons, so Geri and the primary share a row. */}
        {blocker && (
          <p className="order-first flex w-full items-center gap-1.5 text-sm text-muted-foreground sm:order-none sm:ml-auto sm:w-auto" role="status">
            <WarningCircle size={16} className="shrink-0 text-warn" aria-hidden />
            {blocker}
          </p>
        )}
        <div className={cn('ml-auto flex flex-wrap items-center justify-end gap-3', blocker && 'sm:ml-0')}>
          {extra}
          {onPrimary && (
            <Button variant={primaryVariant} onClick={onPrimary} disabled={primaryDisabled || !!blocker || primaryBusy} aria-busy={primaryBusy}>
              {primaryLabel} <ArrowRight aria-hidden />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
