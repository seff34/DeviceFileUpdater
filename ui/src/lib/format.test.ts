import { describe, expect, it } from 'vitest'
import { baseName, formatBytes, formatDuration } from './format'

describe('format', () => {
  it('formats sizes', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1536)).toBe('1,5 KB')
    expect(formatBytes(20 * 1024 * 1024)).toBe('20 MB')
  })
  it('formats durations', () => {
    expect(formatDuration(850)).toBe('850 ms')
    expect(formatDuration(12_400)).toBe('12,4 sn')
    expect(formatDuration(125_000)).toBe('2 dk 5 sn')
  })
  it('takes base names from both separators', () => {
    expect(baseName('files/app.conf')).toBe('app.conf')
    expect(baseName('C:\\x\\y.bin')).toBe('y.bin')
  })
})
