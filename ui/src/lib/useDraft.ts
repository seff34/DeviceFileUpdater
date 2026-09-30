import { useQueryClient, type QueryKey } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'

export type SaveState = 'saved' | 'pending' | 'saving' | 'invalid' | 'error'

interface Options<T> {
  queryKey: QueryKey
  data: T | undefined
  save: (v: T) => Promise<T>
  valid: (v: T) => boolean
  delay?: number
}

/**
 * Local editable copy of server data that saves itself `delay` ms after the
 * last change, but only while valid. Saved results go into the query cache
 * (the wizard gating reads it) and the run status is refetched because any
 * input change can make the dry-run preview stale.
 */
export function useDraft<T>({ queryKey, data, save, valid, delay = 400 }: Options<T>) {
  const qc = useQueryClient()
  const [draft, setLocal] = useState<T | undefined>(undefined)
  const [state, setState] = useState<SaveState>('saved')
  const [error, setError] = useState<string | null>(null)
  const seq = useRef(0)
  const applied = useRef(0)
  const opts = useRef({ queryKey, save, valid, delay })
  useEffect(() => {
    opts.current = { queryKey, save, valid, delay }
  })

  // Seed the local copy from the first server data, during render
  // (adjust-state pattern; effects must not set state).
  if (draft === undefined && data !== undefined) setLocal(data)

  const setDraft = useCallback(
    (next: T) => {
      setLocal(next)
      const my = ++seq.current
      const o = opts.current
      if (!o.valid(next)) {
        setState('invalid')
        return
      }
      setState('pending')
      window.setTimeout(async () => {
        if (my !== seq.current) return
        setState('saving')
        try {
          const saved = await o.save(next)
          if (my > applied.current) {
            applied.current = my
            qc.setQueryData(o.queryKey, saved)
            void qc.invalidateQueries({ queryKey: ['run'] })
          }
          if (my === seq.current) {
            setState('saved')
            setError(null)
          }
        } catch (e) {
          if (my === seq.current) {
            setState('error')
            setError((e as Error).message)
          }
        }
      }, o.delay)
    },
    [qc],
  )

  return { draft: draft ?? data, setDraft, state, error }
}

export function saveBlocker(state: SaveState, error: string | null): string | null {
  if (state === 'pending' || state === 'saving') return 'Kaydediliyor.'
  if (state === 'error') return `Kaydedilemedi: ${error}`
  return null
}
