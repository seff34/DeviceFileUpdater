import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { subscribeRun } from './api'
import { applyEvent, initialRunView, type RunView } from './runState'

/**
 * Follows the current run's event stream while `enabled`. Updates are
 * batched to one render per animation frame so a 200-device run stays smooth.
 *
 * The server replays from the first event on every connection, so the view is
 * rebuilt from scratch whenever the stream (re)opens. The stream closes itself
 * on `run_done`. If it dies before that (for example the session expired,
 * which EventSource cannot report), the run status is refetched so the normal
 * API error path takes over instead of the page hanging.
 */
export function useRunEvents(enabled: boolean, hosts: string[]): RunView | null {
  const qc = useQueryClient()
  const [view, setView] = useState<RunView | null>(null)
  const [wasEnabled, setWasEnabled] = useState(enabled)
  const hostKey = hosts.join('\n')

  // Drop the previous run's view the moment a new stream is wanted.
  if (wasEnabled !== enabled) {
    setWasEnabled(enabled)
    if (enabled) setView(null)
  }

  useEffect(() => {
    if (!enabled) return
    const order = hostKey ? hostKey.split('\n') : []
    let v = initialRunView(order)
    let raf = 0
    let finished = false
    const flush = () => {
      raf = 0
      setView(v)
    }
    const unsubscribe = subscribeRun({
      onReset: () => {
        v = initialRunView(order)
        setView(v)
      },
      onEvent: (e) => {
        v = applyEvent(v, e)
        if (e.type === 'run_done') {
          finished = true
          if (raf) cancelAnimationFrame(raf)
          raf = 0
          setView(v)
          void qc.invalidateQueries({ queryKey: ['run'] })
          void qc.invalidateQueries({ queryKey: ['reports'] })
        } else if (!raf) {
          raf = requestAnimationFrame(flush)
        }
      },
      onClosed: () => {
        if (!finished) void qc.invalidateQueries({ queryKey: ['run'] })
      },
    })
    return () => {
      unsubscribe()
      if (raf) cancelAnimationFrame(raf)
    }
  }, [enabled, hostKey, qc])

  return enabled ? view : null
}
