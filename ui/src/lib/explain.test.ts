import { describe, expect, it } from 'vitest'
import { explainError } from './explain'

describe('explainError', () => {
  it('classifies a combined ssh/telnet refusal as a closed port', () => {
    expect(explainError('Cihaza ulaşılamadı: ssh: ssh connect 127.0.0.1:2: dial tcp 127.0.0.1:2: connect: connection refused; telnet: telnet connect 127.0.0.1:2: dial tcp 127.0.0.1:2: connect: connection refused'))
      .toBe('Cihaz yanıt verdi ama SSH ve Telnet portu kapalı. IP ve portu kontrol edin.')
  })
  it('prefers a wrong password over the other transport failing', () => {
    expect(explainError('connect: authentication failed (ssh: ssh 10.0.0.1:22: authentication failed; telnet: telnet connect 10.0.0.1:23: dial tcp: i/o timeout)'))
      .toBe('Kullanıcı adı veya şifre hatalı.')
  })
  it('explains timeouts, host key changes and engine file errors', () => {
    expect(explainError('dial tcp 10.0.0.9:22: i/o timeout')).toMatch(/zaman aşımı/)
    expect(explainError('host key mismatch: host key for 10.0.0.1 changed')).toMatch(/known_hosts/)
    expect(explainError('device unreachable')).toMatch(/bağlanılamadığı/)
  })
  it('returns empty for unknown or missing errors', () => {
    expect(explainError('something odd')).toBe('')
    expect(explainError(undefined)).toBe('')
  })
})
