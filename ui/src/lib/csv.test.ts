import { describe, expect, it } from 'vitest'
import { parseDeviceText } from './csv'

describe('parseDeviceText', () => {
  it('reads Excel tab-separated paste', () => {
    const r = parseDeviceText('10.0.0.1\troot\tp1\n10.0.0.2\troot\tp2\n')
    expect(r.errors).toEqual([])
    expect(r.devices).toEqual([
      { host: '10.0.0.1', username: 'root', password: 'p1' },
      { host: '10.0.0.2', username: 'root', password: 'p2' },
    ])
  })
  it('reads semicolon CSV with a BOM and header, and quoted fields', () => {
    const r = parseDeviceText('﻿ip;username;password\r\n10.0.0.1;admin;"a;b""c"\r\n')
    expect(r.devices).toEqual([{ host: '10.0.0.1', username: 'admin', password: 'a;b"c' }])
  })
  it('allows an empty password column and reports bad lines', () => {
    const r = parseDeviceText('10.0.0.1,root\n,root,x\n10.0.0.3\n1,2,3,4')
    expect(r.devices).toEqual([{ host: '10.0.0.1', username: 'root', password: '' }])
    expect(r.errors).toEqual([
      'Satır 2: IP gerekli.',
      'Satır 3: en az IP ve kullanıcı adı olmalı.',
      'Satır 4: en fazla 3 sütun olmalı (ip, kullanıcı, şifre).',
    ])
  })
})
