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
  // Newest valid draft not yet sent (null once sent or when the newest draft is invalid).
  const waiting = useRef<{ v: T; seq: number } | null>(null)
  const inflight = useRef(false)
  const timer = useRef<number | null>(null)
  const flushRef = useRef<() => void>(() => {})
  const opts = useRef({ queryKey, save, valid, delay })
  useEffect(() => {
    opts.current = { queryKey, save, valid, delay }
  })

  // Seed the local copy from the first server data, during render
  // (adjust-state pattern; effects must not set state).
  if (draft === undefined && data !== undefined) setLocal(data)
  const ready = data !== undefined

  useEffect(() => {
    // Only one PUT is ever outstanding; when it settles, the latest waiting draft goes next.
    flushRef.current = () => {
      timer.current = null
      const item = waiting.current
      if (inflight.current || !item) return
      waiting.current = null
      inflight.current = true
      setState('saving')
      const o = opts.current
      o.save(item.v)
        .then((saved) => {
          qc.setQueryData(o.queryKey, saved)
          void qc.invalidateQueries({ queryKey: ['run'] })
          if (!waiting.current && item.seq === seq.current) {
            setState('saved')
            setError(null)
          }
        })
        .catch((e: unknown) => {
          if (!waiting.current && item.seq === seq.current) {
            setState('error')
            setError((e as Error).message)
          }
        })
        .finally(() => {
          inflight.current = false
          if (waiting.current && timer.current === null) flushRef.current()
        })
    }
  })

  // Unmount: flush a valid pending draft right away so the cache is current.
  useEffect(
    () => () => {
      if (timer.current !== null) window.clearTimeout(timer.current)
      timer.current = null
      if (waiting.current) flushRef.current()
    },
    [],
  )

  const setDraft = useCallback(
    (next: T) => {
      // Never seed from nothing: before the server data arrives a save would overwrite it.
      if (!ready) return
      setLocal(next)
      const my = ++seq.current
      if (timer.current !== null) window.clearTimeout(timer.current)
      timer.current = null
      if (!opts.current.valid(next)) {
        waiting.current = null
        setState('invalid')
        return
      }
      waiting.current = { v: next, seq: my }
      setState('pending')
      timer.current = window.setTimeout(() => flushRef.current(), opts.current.delay)
    },
    [ready],
  )

  return { draft: draft ?? data, setDraft, state, error }
}

export function saveBlocker(state: SaveState, error: string | null): string | null {
  if (state === 'pending' || state === 'saving') return 'Kaydediliyor.'
  if (state === 'error') return `Kaydedilemedi: ${error}`
  return null
}
