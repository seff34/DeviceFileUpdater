import { Check, Lock } from '@phosphor-icons/react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { navigate } from '@/lib/router'
import { STEPS, blocker, stepStates, type StepId } from '@/lib/steps'
import { useWizard } from '@/wizard/WizardContext'
import { cn } from '@/lib/utils'

export function Stepper({ current }: { current: StepId }) {
  const { facts } = useWizard()
  const states = stepStates(current, facts)
  const ci = STEPS.findIndex((s) => s.id === current)
  return (
    <nav aria-label="Sihirbaz adımları" className="border-b bg-card">
      <p className="px-6 py-2 text-sm text-muted-foreground md:hidden">
        Adım {ci + 1}/{STEPS.length}: <span className="font-medium text-foreground">{STEPS[ci].title}</span>
      </p>
      <ol className="mx-auto hidden max-w-6xl grid-cols-7 px-6 md:grid">
        {STEPS.map((s, i) => {
          const st = states[s.id]
          const prevBlock = i > 0 ? STEPS.slice(0, i).map((p) => blocker(p.id, facts)).find(Boolean) : null
          const item = (
            <button
              type="button"
              disabled={st === 'blocked' || st === 'current'}
              aria-current={st === 'current' ? 'step' : undefined}
              onClick={() => navigate(s.path)}
              className={cn(
                'group flex w-full items-center gap-2 border-b-2 py-3 text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring',
                st === 'current' && 'border-primary font-medium text-foreground',
                st === 'done' && 'border-transparent text-foreground hover:border-border',
                st === 'reachable' && 'border-transparent text-muted-foreground hover:text-foreground',
                st === 'blocked' && 'cursor-not-allowed border-transparent text-muted-foreground/60',
              )}
            >
              <span
                className={cn(
                  'flex size-6 shrink-0 items-center justify-center rounded-md border font-mono text-xs',
                  st === 'current' && 'border-primary bg-primary text-primary-foreground',
                  st === 'done' && 'border-foreground/20 bg-foreground text-background',
                )}
              >
                {st === 'done' ? <Check size={14} weight="bold" aria-hidden /> : st === 'blocked' ? <Lock size={12} aria-hidden /> : i + 1}
              </span>
              <span className="truncate">{s.title}</span>
              {st === 'done' && <span className="sr-only">(tamamlandı)</span>}
            </button>
          )
          return (
            <li key={s.id}>
              {st === 'blocked' && prevBlock ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="block">{item}</span>
                  </TooltipTrigger>
                  <TooltipContent>{prevBlock}</TooltipContent>
                </Tooltip>
              ) : (
                item
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
