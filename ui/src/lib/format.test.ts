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
  it('clamps negatives and handles zero', () => {
    expect(formatBytes(-5)).toBe('0 B')
    expect(formatBytes(0)).toBe('0 B')
    expect(formatDuration(-1)).toBe('0 ms')
    expect(formatDuration(0)).toBe('0 ms')
  })
  it('rolls over units at rounding boundaries', () => {
    expect(formatBytes(1023)).toBe('1023 B')
    expect(formatBytes(1024 * 1024 - 1)).toBe('1 MB')
    expect(formatBytes(1024 ** 3 - 1)).toBe('1 GB')
    expect(formatBytes(999.6)).toBe('1000 B')
    expect(formatDuration(999.6)).toBe('1 sn')
    expect(formatDuration(59_950)).toBe('1 dk')
    expect(formatDuration(59_940)).toBe('59,9 sn')
    expect(formatDuration(60_000)).toBe('1 dk')
    expect(formatDuration(119_600)).toBe('2 dk')
  })
  it('handles huge values', () => {
    expect(formatBytes(5 * 1024 ** 4)).toBe('5 TB')
    expect(formatBytes(4096 * 1024 ** 4)).toBe('4.096 TB')
    expect(formatDuration(36_000_000)).toBe('600 dk')
  })
  it('takes base names from both separators', () => {
    expect(baseName('files/app.conf')).toBe('app.conf')
    expect(baseName('C:\\x\\y.bin')).toBe('y.bin')
  })
})
