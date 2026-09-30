import { expect, test } from '@playwright/test'
import fs from 'node:fs'
import path from 'node:path'

interface Fleet {
  url: string
  token: string
  root: string
  parent: string
  down: string
  devices: { host: string; username: string; password: string }[]
}
// Written by test/e2e/fakefleet. It holds fixture passwords, so it is never printed.
// Matches a table row whose name starts with this host and not a longer port with the same prefix.
const rowOf = (host: string) => new RegExp(`^${host.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}(?!\\d)`)
const fleet = (): Fleet => JSON.parse(fs.readFileSync(path.join(import.meta.dirname, '.bin', 'fleet.json'), 'utf8'))

test('operator updates a fleet end to end without a terminal', async ({ page }) => {
  const f = fleet()
  const healthy = f.devices.length - 1

  // 1. Launch URL with token lands on the workspace step.
  await page.goto(`/?token=${f.token}`)
  await expect(page.getByRole('heading', { level: 1, name: 'Çalışma alanı', exact: true })).toBeVisible()
  const address = page.getByLabel('Klasör yolu')
  // The address bar is overwritten once when the first listing arrives; wait for that.
  await expect(address).not.toHaveValue('')
  await address.fill(f.parent)
  await page.getByRole('button', { name: 'Git' }).click()
  // The parent folder is empty, so this text means the new listing replaced the old one.
  await expect(page.getByText('Bu klasörde alt klasör yok.')).toBeVisible()
  await page.getByLabel('Yeni klasör adı').fill('hat-1')
  await page.getByRole('button', { name: 'Oluştur ve kullan' }).click()
  await expect(page.getByRole('heading', { name: 'Açık çalışma alanı' })).toBeVisible()
  await page.getByRole('button', { name: /Devam/ }).click()

  // 2. Devices: bulk paste, then a connection test flags the closed port.
  await expect(page).toHaveURL(/\/devices$/)
  // The empty state repeats this button, hence first().
  await page.getByRole('button', { name: 'Toplu ekle' }).first().click()
  await page.getByLabel('Cihaz satırları').fill(f.devices.map((d) => `${d.host}\t${d.username}\t${d.password}`).join('\n'))
  await page.getByRole('button', { name: `Listeye ekle (${f.devices.length})` }).click()
  await page.getByRole('button', { name: 'Tümünü test et' }).click()
  await expect(page.getByText('Cihaza ulaşılamadı')).toBeVisible()
  await expect(page.getByText(/1 cihaza bağlanılamadı/)).toBeVisible()
  await expect(page.getByRole('button', { name: 'Ulaşılamayanları çıkar' })).toBeVisible()
  await expect(page.getByText('telnet', { exact: true })).toHaveCount(healthy)
  await page.getByRole('button', { name: /Devam/ }).click()

  // 3. Files: upload, folder target completes to the file name, custom mode.
  await expect(page).toHaveURL(/\/files$/)
  await page.getByLabel('Dosya yükle').setInputFiles({ name: 'app.conf', mimeType: 'text/plain', buffer: Buffer.from('v=1\n') })
  const remote = page.getByLabel('Hedef yol, satır 1')
  await remote.fill('/devroot/etc/app/')
  await remote.press('Tab')
  await expect(remote).toHaveValue('/devroot/etc/app/app.conf')
  await page.getByLabel('İzin, satır 1').fill('0640')
  await page.getByRole('button', { name: /Devam/ }).click()

  // 4. Settings: shorter connect timeout keeps the unreachable device quick.
  await expect(page).toHaveURL(/\/settings$/)
  await page.getByRole('button', { name: /Gelişmiş/ }).click()
  await page.getByLabel('Bağlantı zaman aşımı (sn)').fill('2')
  await expect(page.getByText('Kaydedildi')).toBeVisible()
  // The value reached the server: it survives a reload.
  await page.reload()
  await page.getByRole('button', { name: /Gelişmiş/ }).click()
  await expect(page.getByLabel('Bağlantı zaman aşımı (sn)')).toHaveValue('2')
  await page.getByRole('button', { name: /Devam/ }).click()

  // 5. Preview is mandatory and needs explicit confirmation.
  await expect(page).toHaveURL(/\/preview$/)
  await expect(page.getByText('Önce güncel bir önizleme çalıştırın.')).toBeVisible()
  await page.getByRole('button', { name: 'Önizlemeyi başlat' }).click()
  await expect(page.getByText(`${healthy} cihazda ${healthy} dosya oluşturulacak. 1 cihaza ulaşılamadı.`)).toBeVisible({ timeout: 60_000 })
  // A preview writes nothing to the devices.
  expect(fs.existsSync(path.join(f.root, 'dev-0', 'etc', 'app', 'app.conf'))).toBe(false)
  await page.getByRole('checkbox', { name: /Değişiklikleri inceledim/ }).check()
  await page.getByRole('button', { name: /Uygulamayı başlat/ }).click()

  // 6. Apply streams to completion.
  await expect(page).toHaveURL(/\/apply$/)
  await expect(page.getByText(`${f.devices.length} cihazdan ${healthy} tanesi başarılı, 1 tanesi başarısız.`, { exact: false })).toBeVisible({ timeout: 90_000 })
  // The live grid keeps every row: the down host failed, a healthy one is done with its file created.
  await expect(page.getByRole('row', { name: rowOf(f.down) })).toContainText('Başarısız')
  const live = page.getByRole('row', { name: rowOf(f.devices[0].host) })
  await expect(live).toContainText('Tamamlandı')
  await expect(live).toContainText('1 oluşturuldu')
  for (let i = 0; i < healthy; i++) {
    const p = path.join(f.root, `dev-${i}`, 'etc', 'app', 'app.conf')
    expect(fs.readFileSync(p, 'utf8')).toBe('v=1\n')
    expect(fs.statSync(p).mode & 0o777).toBe(0o640)
  }
  await page.getByRole('button', { name: /Raporu gör/ }).click()

  // 7. Report: failure visible, retry asks first and counts hosts, HTML report has no passwords.
  await expect(page).toHaveURL(/\/report$/)
  await expect(page.getByRole('row', { name: rowOf(f.down) })).toContainText('Başarısız')
  await expect(page.getByRole('row', { name: rowOf(f.devices[0].host) })).toContainText('Başarılı')
  await page.getByRole('button', { name: /Başarısızları tekrar dene/ }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toContainText(`Yalnızca 1 cihazda yeni bir uygulama başlatılır: ${f.down}`)
  await dialog.getByRole('button', { name: 'Vazgeç' }).click()
  await expect(dialog).toBeHidden()
  const href = await page.getByRole('link', { name: /HTML raporu aç/ }).getAttribute('href')
  const html = await (await page.request.get(href!)).text()
  expect(html).toContain(f.down)
  for (const d of f.devices) expect(html).not.toContain(d.password)
  const reports = path.join(f.parent, 'hat-1', 'reports')
  const written = fs.readdirSync(reports)
  expect(written.length).toBeGreaterThan(0)
  for (const name of written)
    for (const d of f.devices) expect(fs.readFileSync(path.join(reports, name), 'utf8')).not.toContain(d.password)

  // 8. A second preview proves idempotence.
  await page.getByRole('button', { name: /Yeni önizleme/ }).click()
  await page.getByRole('button', { name: /Önizlemeyi (başlat|yeniden çalıştır)/ }).click()
  await expect(page.getByText(`Hiçbir cihazda değişiklik yok. ${healthy} cihaz zaten güncel, 1 cihaza ulaşılamadı.`)).toBeVisible({ timeout: 60_000 })

  // 9. History lists all three runs, newest first.
  await page.getByRole('link', { name: /Geçmiş raporlar/ }).click()
  const runs = page.getByRole('main').getByRole('list').getByRole('link')
  await expect(runs).toHaveCount(3)
  await expect(runs.first()).toContainText('Önizleme')
  await expect(runs.filter({ hasText: 'Uygulama' })).toContainText('1 başarısız')
})

test('a session without the token is refused', async ({ browser }) => {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  await page.goto('/')
  await expect(page.getByText('Oturum bulunamadı')).toBeVisible()
  await ctx.close()
})
