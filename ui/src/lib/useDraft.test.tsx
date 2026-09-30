import { describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useDraft } from './useDraft'

function setup(save: (v: string[]) => Promise<string[]>, data: string[] | null = ['a']) {
  const qc = new QueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  const hook = renderHook(
    () => useDraft<string[]>({ queryKey: ['x'], data: data ?? undefined, save, valid: (v) => v.every((s) => s !== ''), delay: 10 }),
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
  it('ignores setDraft until data has arrived', async () => {
    const save = vi.fn(async (v: string[]) => v)
    const { hook } = setup(save, null)
    act(() => hook.result.current.setDraft(['x']))
    expect(hook.result.current.draft).toBeUndefined()
    await new Promise((r) => setTimeout(r, 30))
    expect(save).not.toHaveBeenCalled()
  })
  it('never overlaps saves and sends only the latest draft after a slow one', async () => {
    let active = 0
    let maxActive = 0
    const bodies: string[][] = []
    let release!: () => void
    const save = vi.fn(async (v: string[]) => {
      active++
      maxActive = Math.max(maxActive, active)
      bodies.push(v)
      if (bodies.length === 1) await new Promise<void>((r) => (release = r))
      active--
      return v
    })
    const { qc, hook } = setup(save)
    act(() => hook.result.current.setDraft(['b']))
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1))
    act(() => hook.result.current.setDraft(['c']))
    act(() => hook.result.current.setDraft(['d']))
    await new Promise((r) => setTimeout(r, 40))
    expect(save).toHaveBeenCalledTimes(1)
    expect(hook.result.current.state).not.toBe('saved')
    await act(async () => release())
    await waitFor(() => expect(hook.result.current.state).toBe('saved'))
    expect(save).toHaveBeenCalledTimes(2)
    expect(maxActive).toBe(1)
    expect(bodies.at(-1)).toEqual(['d'])
    expect(qc.getQueryData(['x'])).toEqual(['d'])
  })
  it('flushes a pending valid draft on unmount', async () => {
    const save = vi.fn(async (v: string[]) => v)
    const { qc, hook } = setup(save)
    act(() => hook.result.current.setDraft(['z']))
    hook.unmount()
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(qc.getQueryData(['x'])).toEqual(['z']))
  })
})
