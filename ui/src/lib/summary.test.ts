import { describe, expect, it } from 'vitest'
import { previewSentence, resultSentence, tally } from './summary'
import type { RunResult } from './types'

const f = (remote: string, status: RunResult['devices'][0]['files'][0]['status']) => ({ remote, status, duration_ms: 1 })
const preview: RunResult = {
  id: 'p', started: '', finished: '', dry_run: true, parallel: 4, files: ['/a', '/b'],
  devices: [
    { host: 'h1', duration_ms: 1, files: [f('/a', 'WOULD_CREATE'), f('/b', 'WOULD_CREATE')] },
    { host: 'h2', duration_ms: 1, files: [f('/a', 'WOULD_UPDATE'), f('/b', 'UNCHANGED')] },
    { host: 'h3', duration_ms: 1, files: [f('/a', 'UNCHANGED'), f('/b', 'UNCHANGED')] },
    { host: 'h4', duration_ms: 1, error: 'Cihaza ulaşılamadı', files: [f('/a', 'FAILED'), f('/b', 'FAILED')] },
  ],
}

describe('summary', () => {
  it('tallies devices and files', () => {
    expect(tally(preview)).toMatchObject({ devices: 4, devCreate: 1, devUpdate: 1, filesCreate: 2, filesUpdate: 1, upToDate: 1, unreachable: 1, fileErrors: 0, changedDevices: 2 })
  })
  it('writes the preview sentence', () => {
    expect(previewSentence(preview)).toBe('1 cihazda 2 dosya oluşturulacak, 1 cihazda 1 dosya güncellenecek. 1 cihaz zaten güncel, 1 cihaza ulaşılamadı.')
  })
  it('says when nothing changes', () => {
    const same = { ...preview, devices: [preview.devices[2]] }
    expect(previewSentence(same)).toBe('Hiçbir cihazda değişiklik yok. 1 cihaz zaten güncel.')
  })
  it('writes the result sentence', () => {
    const done: RunResult = {
      ...preview, dry_run: false,
      devices: [
        { host: 'h1', duration_ms: 1, files: [f('/a', 'CREATED'), f('/b', 'UPDATED')] },
        { host: 'h2', duration_ms: 1, files: [f('/a', 'UNCHANGED'), f('/b', 'FAILED')] },
      ],
    }
    expect(resultSentence(done)).toBe('2 cihazdan 1 tanesi başarılı, 1 tanesi başarısız. 1 dosya oluşturuldu, 1 dosya güncellendi, 1 dosya aynıydı, 1 dosya başarısız.')
  })
  it('does not claim success for a report with no devices', () => {
    expect(resultSentence({ ...preview, dry_run: false, devices: [] })).toBe('Hiçbir cihaz işlenmedi.')
  })
})
