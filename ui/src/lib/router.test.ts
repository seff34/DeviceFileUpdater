import { describe, expect, it } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { navigate, usePath, reportIdFromPath } from './router'

describe('router', () => {
  it('updates subscribers on navigate and back', () => {
    const { result } = renderHook(() => usePath())
    act(() => navigate('/devices'))
    expect(result.current).toBe('/devices')
    act(() => navigate('/files'))
    expect(result.current).toBe('/files')
    act(() => {
      window.history.replaceState(null, '', '/devices')
      window.dispatchEvent(new PopStateEvent('popstate'))
    })
    expect(result.current).toBe('/devices')
  })
  it('extracts report ids', () => {
    expect(reportIdFromPath('/history/20260930-101010-ab12')).toBe('20260930-101010-ab12')
    expect(reportIdFromPath('/history')).toBeNull()
  })
})
