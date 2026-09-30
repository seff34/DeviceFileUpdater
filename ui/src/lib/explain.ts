/**
 * Turns a raw engine error (Go, English, often "ssh: ...; telnet: ...") into one
 * Turkish sentence that tells the technician what went wrong and what to check.
 * Returns '' when the error is not recognised; callers then show the raw text only.
 * Order matters: a combined ssh/telnet error is classified by its most specific cause.
 */
const RULES: [RegExp, string][] = [
  [/authentication failed|unable to authenticate|login incorrect/i, 'Kullanıcı adı veya şifre hatalı.'],
  [/host key (changed|mismatch)/i, "Cihazın SSH anahtarı değişmiş. Cihaz yeniden kurulduysa çalışma alanındaki known_hosts dosyasından satırını silin."],
  [/no (login|password|shell) prompt/i, 'Telnet girişi tamamlanamadı: cihaz beklenen giriş ekranını göstermedi.'],
  [/no such host/i, 'Adres çözülemedi. IP adresini kontrol edin.'],
  [/no route to host|network is unreachable|host is down/i, 'Cihaza ağ üzerinden ulaşılamıyor. Bilgisayarın cihaz ağına bağlı olduğunu kontrol edin.'],
  [/connection refused/i, 'Cihaz yanıt verdi ama SSH ve Telnet portu kapalı. IP ve portu kontrol edin.'],
  [/i\/o timeout|deadline exceeded|timed out|timeout/i, 'Cihaz yanıt vermedi (zaman aşımı). IP adresini ve ağ bağlantısını kontrol edin.'],
  [/device unreachable/i, 'Cihaza bağlanılamadığı için dosya gönderilmedi.'],
  [/probe failed/i, 'Cihazdaki araçlar tespit edilemediği için dosya gönderilmedi.'],
  [/target is a symlink/i, 'Hedef yol bir sembolik bağlantı. Manifestte gerçek yolu yazın.'],
  [/not a regular file/i, 'Hedef yolda dosya yerine bir klasör veya özel dosya var.'],
  [/hash mismatch/i, 'Dosya gönderildi ama cihazdaki içerik doğrulanamadı.'],
  [/no upload method|all upload methods failed/i, 'Cihazda kullanılabilir bir yükleme yöntemi bulunamadı.'],
  [/permission denied|read-only file system/i, 'Cihazda bu yola yazma izni yok.'],
  [/no space left/i, 'Cihazda yer kalmadı.'],
]

export function explainError(raw: string | undefined): string {
  if (!raw) return ''
  for (const [re, text] of RULES) if (re.test(raw)) return text
  return ''
}
