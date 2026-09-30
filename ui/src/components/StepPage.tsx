import type { ReactNode } from 'react'
import { ActionBar, type ActionConfig } from './ActionBar'
import { stepById, type StepId } from '@/lib/steps'

export function StepPage({ step, action, children, aside }: { step: StepId; action: ActionConfig; children: ReactNode; aside?: ReactNode }) {
  const s = stepById(step)
  return (
    <>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{s.title}</h1>
            <p className="mt-1 max-w-[65ch] text-sm text-muted-foreground">{s.purpose}</p>
          </div>
          {aside}
        </header>
        {children}
      </main>
      <ActionBar {...action} />
    </>
  )
}
