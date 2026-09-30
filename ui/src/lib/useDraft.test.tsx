import { describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useDraft } from './useDraft'

function setup(save: (v: string[]) => Promise<string[]>) {
  const qc = new QueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  const hook = renderHook(
    () => useDraft<string[]>({ queryKey: ['x'], data: ['a'], save, valid: (v) => v.every((s) => s !== ''), delay: 10 }),
    { wrapper },
  )
  return { qc, hook }
}

describe('useDraft', () => {
  it('saves valid drafts after the delay and publishes them to the cache', async () => {
    const save = vi.fn(async (v: string[]) => v)
    const { qc, hook } = setup(save)
    expect(hook.result.current.draft).toEqual(['a'])
    act(() => hook.result.current.setDraft(['a', 'b']))
    expect(hook.result.current.state).toBe('pending')
    await waitFor(() => expect(hook.result.current.state).toBe('saved'))
    expect(save).toHaveBeenCalledTimes(1)
    expect(qc.getQueryData(['x'])).toEqual(['a', 'b'])
  })
  it('keeps invalid drafts local', async () => {
    const save = vi.fn(async (v: string[]) => v)
    const { hook } = setup(save)
    act(() => hook.result.current.setDraft(['a', '']))
    expect(hook.result.current.state).toBe('invalid')
    await new Promise((r) => setTimeout(r, 30))
    expect(save).not.toHaveBeenCalled()
  })
  it('coalesces rapid edits and surfaces save errors', async () => {
    const save = vi.fn(async () => {
      throw new Error('devices.csv yazılamadı')
    })
    const { hook } = setup(save)
    act(() => {
      hook.result.current.setDraft(['b'])
      hook.result.current.setDraft(['c'])
    })
    await waitFor(() => expect(hook.result.current.state).toBe('error'))
    expect(save).toHaveBeenCalledTimes(1)
    expect(hook.result.current.error).toBe('devices.csv yazılamadı')
  })
})
