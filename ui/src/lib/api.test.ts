import { describe, expect, it, vi, afterEach } from 'vitest'
import { api, ApiError, readNdjson } from './api'

function streamOf(chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream({
    start(c) {
      chunks.forEach((ch) => c.enqueue(enc.encode(ch)))
      c.close()
    },
  })
}

afterEach(() => vi.restoreAllMocks())

describe('readNdjson', () => {
  it('parses lines split across chunks', async () => {
    const got: unknown[] = []
    await readNdjson(streamOf(['{"host":"a"', ',"ok":true}\n{"ho', 'st":"b"}\n', '{"done":true}']), (v) => got.push(v))
    expect(got).toEqual([{ host: 'a', ok: true }, { host: 'b' }, { done: true }])
  })
})

describe('request', () => {
  it('turns error JSON into ApiError with the server message', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Önce bir çalışma alanı seçin.' }), { status: 409 }),
    )
    await expect(api.devices()).rejects.toEqual(new ApiError(409, 'Önce bir çalışma alanı seçin.'))
  })
  it('sends JSON bodies with the content type', async () => {
    const spy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ devices: [] }), { status: 200 }))
    await api.saveDevices([{ host: '10.0.0.1', username: 'root', password: 'x' }])
    const [url, init] = spy.mock.calls[0]
    expect(url).toBe('/api/devices')
    expect(init?.method).toBe('PUT')
    expect((init?.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(JSON.parse(init?.body as string)).toEqual({ devices: [{ host: '10.0.0.1', username: 'root', password: 'x' }] })
  })
  it('returns undefined for 204', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
    await expect(api.cancelRun()).resolves.toBeUndefined()
  })
})
