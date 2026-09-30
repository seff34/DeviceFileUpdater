import { createContext, useContext, useState, type ReactNode } from 'react'
import { useWizardFacts } from './useWizardFacts'
import type { WizardFacts } from '@/lib/steps'

interface WizardValue {
  facts: WizardFacts
  loading: boolean
  previewConfirmed: boolean
  setPreviewConfirmed: (v: boolean) => void
}

const Ctx = createContext<WizardValue | null>(null)

export function WizardProvider({ children }: { children: ReactNode }) {
  const [previewConfirmed, setPreviewConfirmed] = useState(false)
  const { facts, loading } = useWizardFacts(previewConfirmed)
  const previewId = facts.run?.preview_id ?? ''
  const fresh = facts.run?.preview_fresh ?? false
  // A new or stale preview needs a fresh confirmation. Reset during render
  // (React's "adjusting state when a prop changes" pattern) instead of in an effect.
  const previewKey = `${previewId}|${fresh}`
  const [seenPreviewKey, setSeenPreviewKey] = useState(previewKey)
  if (seenPreviewKey !== previewKey) {
    setSeenPreviewKey(previewKey)
    setPreviewConfirmed(false)
  }
  return <Ctx.Provider value={{ facts, loading, previewConfirmed, setPreviewConfirmed }}>{children}</Ctx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components -- the hook belongs with its provider
export function useWizard() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useWizard outside WizardProvider')
  return v
}
