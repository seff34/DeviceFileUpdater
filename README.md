# DeviceFileUpdater

Gömülü Linux cihazlarına SSH veya Telnet üzerinden dosya gönderen, tek dosyalık bir araç. Tarayıcıda açılan adım adım bir arayüzü vardır; aynı motor komut satırından da kullanılabilir.

Onlarca veya yüzlerce cihaza aynı dosya setini (yapılandırma, betik, yazılım parçası) dağıtmak ve sonucu raporla kanıtlamak için yapılmıştır. İnternet gerektirmez; bilgisayarın cihaz ağına bağlı olması yeterlidir.

## Özellikler

- **Protokol ve yöntemi kendisi seçer.** Her cihaz için SSH veya Telnet'i, ardından kullanılabilir yükleme yöntemini bulur: `sftp`, `scp`, `ftp`; bunlar yoksa kabuk üzerinden `shell-base64` veya `shell-printf`.
- **Araçsız cihazlarda da çalışır.** Cihazda hash aracı yoksa (`sha256sum`, `md5sum`) içeriği `base64`, `od` veya `hexdump` ile geri okuyarak karşılaştırır.
- **Yalnız gerekeni yazar.** Aynı olan dosyaya dokunmaz, farklı olanı günceller, eksik olanı oluşturur.
- **Güvenli yazma.** Dosya önce geçici bir dosyaya yüklenir, doğrulanır, sonra yerine taşınır. İsteğe bağlı olarak eski dosyanın yedeği (`<hedef>.bak`) alınır.
- **Zorunlu önizleme.** Uygulamadan önce bir deneme çalıştırması (dry-run) her cihazda neyin değişeceğini gösterir. Önizlemeden sonra cihaz, dosya veya ayar değişirse önizleme yeniden istenir.
- **Canlı izleme ve rapor.** Her cihazın aşaması canlı izlenir. Her çalıştırma JSON ve HTML rapor yazar; başarısız cihazlar tek tuşla yeniden denenir.
- **post-command.** Dosyalardan sonra her cihazda bir komut çalıştırılabilir (örn. servisi yeniden başlatmak); yalnız değişiklik olduğunda veya her zaman.
- **Excel uyumlu dosyalar.** Cihaz ve dosya listeleri Türkçe Excel'in açtığı biçimde (`;` ile ayrılmış, UTF-8 BOM) yazılır; `,` ile ayrılmış dosyalar da okunur.
- **Tek dosya, kurulum yok.** Windows, macOS ve Linux için tek çalıştırılabilir dosya. Arayüz programın içine gömülüdür; CDN veya uzak yazı tipi kullanmaz.

## Hızlı başlangıç (operatör)

1. Platformunuza uygun paketi indirip açın (örn. `devupdater-windows-amd64.zip`).
2. `devupdater` (Windows'ta `devupdater.exe`) dosyasına çift tıklayın. Tarayıcı kendiliğinden açılır.
3. Arayüzdeki yedi adımı izleyin:
   1. **Çalışma alanı:** bir klasör seçin veya oluşturun.
   2. **Cihazlar:** cihazları ekleyin (Excel'den yapıştırabilirsiniz) ve bağlantıyı test edin.
   3. **Dosyalar:** gönderilecek dosyaları ve cihazdaki hedef yollarını girin.
   4. **Ayarlar:** paralel cihaz sayısı, yedek, post-command.
   5. **Önizleme:** neyin değişeceğini görün ve onaylayın.
   6. **Uygula:** dosyalar gönderilir, her cihaz canlı izlenir.
   7. **Rapor:** sonuçlar; başarısızları tekrar deneyin.

Ayrıntılı operatör kılavuzu her pakette `KULLANIM.txt` olarak bulunur ve arayüzde sağ üstteki **Kullanım kılavuzu** bağlantısından açılır.

Programlar imzasızdır. Windows "bilgisayarınızı korudu", macOS "geliştirici doğrulanamadı" uyarısı verebilir; nasıl geçileceği `KULLANIM.txt` içinde anlatılır.

## Çalışma alanı

Bir hat veya iş için her şey tek klasörde tutulur:

```
hat-3-guncelleme/
├── devices.csv     cihaz listesi
├── manifest.csv    gönderilecek dosyalar
├── settings.json   ayarlar
├── files/          gönderilecek dosyaların kendisi
└── reports/        her çalıştırmanın JSON ve HTML raporu
```

`devices.csv`:

```
ip;username;password
192.168.1.101;root;sifre
192.168.1.103:2222;admin;sifre
```

`manifest.csv` (`mode` boş bırakılırsa var olan dosyanın izni korunur):

```
local_path;remote_path;mode
files/uygulama.conf;/etc/uygulama/uygulama.conf;0644
```

Örnek bir çalışma alanı [`packaging/ornek-calisma-alani`](packaging/ornek-calisma-alani) altındadır.

## Komut satırı

```
devupdater                                   arayüzü açar (devupdater ui ile aynı)
devupdater ui  [-workspace DIR] [-port N] [-no-browser]
devupdater run [-workspace DIR] [-dry-run] [-parallel N] [-only-failed REPORT_ID]
```

`run` çıkış kodları: `0` tüm cihazlar başarılı, `1` bazı cihazlar başarısız, `2` kullanım veya yapılandırma hatası.

## Güvenlik

- Arayüz yalnız `127.0.0.1` üzerinde dinler ve her başlatmada yeni bir erişim anahtarı (token) üretir. Ağdaki başka bir bilgisayardan açılamaz.
- `devices.csv` şifreleri düz metin tutar ve `0600` izniyle yazılır. Çalışma alanını yalnız yetkili kişilerle paylaşın.
- Şifreler raporlara, günlüklere ve konsol çıktısına yazılmaz.
- SSH anahtar doğrulaması varsayılan olarak kapalıdır (`strict_host_key: false`); izole cihaz ağları için seçilmiş bir varsayılandır. `settings.json` içinde `strict_host_key: true` yapılırsa her cihazın anahtarı ilk bağlantıda çalışma alanındaki `known_hosts` dosyasına kaydedilir ve sonradan değişen anahtar reddedilir.

## Kaynaktan derleme

Gerekenler: Go 1.26+, Node.js (arayüz için), `zip`.

```bash
make ui         # arayüz bağımlılıklarını kurar (npm ci) ve derler
make release    # tüm platformlar için dist/ altında paket ve zip üretir
```

Yalnız kendi platformunuz için:

```bash
make ui-build
go build -o devupdater ./cmd/devupdater
```

## Testler

```bash
make test           # go vet ve go test -race
cd ui && npm test   # arayüz birim testleri (Vitest)
make e2e            # Playwright ile uçtan uca test, sahte cihaz filosuna karşı
make integration    # Docker ile gerçek SSH/Telnet/FTP cihaz kapları (Docker gerekir)
```

`make e2e` için Chromium bir kez kurulmalıdır: `cd ui && npx playwright install chromium`.

## Proje yapısı

```
cmd/devupdater/      giriş noktası
internal/cli/        ui ve run komutları
internal/web/        HTTP API, SSE olayları, gömülü arayüz
internal/runner/     çalıştırma motoru (paralel cihazlar, iptal, post-command)
internal/transport/  SSH ve Telnet oturumları
internal/probe/      cihazdaki araçların tespiti
internal/upload/     yükleme yöntemleri (sftp, scp, ftp, kabuk)
internal/syncer/     karşılaştırma, geçici dosyaya yazma, doğrulama
internal/workspace/  CSV ve ayar dosyaları
internal/report/     JSON ve HTML rapor
ui/                  React arayüzü (Vite, TypeScript, Tailwind)
packaging/           pakete giren kılavuz ve örnek çalışma alanı
test/                sahte filo (e2e) ve Docker entegrasyon testleri
docs/                tasarım belgesi ve uygulama planları
```

Ürün ve tasarım ilkeleri [`PRODUCT.md`](PRODUCT.md) ve [`DESIGN.md`](DESIGN.md) içindedir.

## Lisans

[MIT](LICENSE)
