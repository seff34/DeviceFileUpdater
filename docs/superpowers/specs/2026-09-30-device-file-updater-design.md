# DeviceFileUpdater — Tasarım Dokümanı

Tarih: 2026-09-30
Durum: Onaylandı (brainstorming), implementasyon planı bekliyor

## 1. Amaç

Host bilgisayarda çalışan, cross-platform bir uygulama. Embedded Linux cihazlara SSH veya Telnet ile bağlanıp, tanımlı yerel dosyaları cihazdaki tanımlı path'lere kopyalar.

Temel kurallar:

- Cihaz listesi CSV'den okunur (IP, kullanıcı adı, şifre).
- SSH ve Telnet desteklenir; protokol cihaz başına otomatik seçilir.
- Cihazda SCP/SFTP/FTP olsa da olmasa da çalışır.
- Cihazdaki dosya yerel dosyayla aynıysa dokunulmaz; farklıysa güncellenir; yoksa oluşturulur.
- Her çalıştırmada rapor üretilir.
- Windows, Linux, macOS üzerinde çalışır.
- Operatör her şeyi adım adım yönlendiren bir web UI üzerinden yapabilir; otomasyon için aynı motoru kullanan bir CLI da vardır.

## 2. Kapsam dışı

- `sudo`/yetki yükseltme. Login kullanıcısının hedef path'e yazma yetkisi olduğu varsayılır; yoksa dosya `FAILED` olarak raporlanır.
- Cihazdan hosta dosya çekme, cihazdan dosya silme.
- Cihaz başına farklı dosya seti (tüm cihazlara aynı manifest uygulanır).
- Çok kullanıcılı / uzaktan erişilen UI (UI sadece `127.0.0.1`).

## 3. Teknoloji

- **Backend:** Go, CGO kapalı, tek statik binary.
  - SSH: `golang.org/x/crypto/ssh`, SFTP: `github.com/pkg/sftp`.
  - Telnet: kendi istemcimiz (IAC negotiation, login prompt, marker'lı komut çıktısı).
  - FTP istemcisi: `github.com/jlaffaye/ftp`.
- **Frontend:** React + TypeScript + Vite, Tailwind + shadcn/ui, TanStack Query, SSE.
  - Build çıktısı `embed.FS` ile binary'ye gömülür. Node sadece build sırasında gerekir.
- **Release hedefleri:** windows/linux/darwin × amd64/arm64 (+ linux/arm isteğe bağlı).

## 4. Mimari

```
devupdater (tek binary)
├── cmd        `devupdater ui [-workspace DIR]`   → localhost UI, tarayıcıyı açar (argümansız çalıştırma = ui)
│              `devupdater run -workspace DIR [-dry-run] [-parallel N] [-only-failed REPORT]`
├── workspace  devices.csv, manifest.csv, settings.json, files/, reports/ okur/yazar, doğrular
├── transport  Session arayüzü: Exec(ctx, cmd) (stdout, exitCode, err) / Close
│   ├── ssh      exec kanalı; exec desteklenmezse PTY shell + marker modu
│   └── telnet   login + PTY shell + marker modu
├── uploader   yükleme yöntemleri: sftp, scp, ftp, shell-base64, shell-printf
├── probe      bağlantı sonrası yetenek tespiti
├── sync       dosya başına karar + atomik yazma
├── runner     worker pool, cihaz izolasyonu, iptal, event yayını
├── report     HTML rapor + konsol özeti + makine okur run kaydı (JSON, retry için)
└── web        REST API + SSE + gömülü React uygulaması
```

Birimler arası sınırlar:

- `sync` sadece `Session` + `Uploader` arayüzlerini bilir; SSH/Telnet ayrımını bilmez. Böylece sahte Session ile unit test edilir.
- `runner` event'leri bir kanala yayınlar; CLI bunları konsola, UI SSE'ye basar.
- `web` sadece `workspace` ve `runner` API'lerini çağırır; iş mantığı içermez.

## 5. Çalışma alanı (workspace)

```
workspace/
├── devices.csv     ip,username,password
├── manifest.csv    local_path,remote_path,mode
├── settings.json
├── files/          UI'dan eklenen dosyaların kopyaları
└── reports/        YYYYMMDD-HHMMSS.html + YYYYMMDD-HHMMSS.json
```

### devices.csv

```
ip,username,password
192.168.1.10,root,secret
192.168.1.11,admin,secret2
```

- Başlık satırı zorunlu. IP isteğe bağlı olarak `ip:port` olabilir (varsayılanlar SSH 22, Telnet 23).
- Tekrar eden IP → doğrulama hatası.

### manifest.csv

```
local_path,remote_path,mode
files/app.bin,/opt/app/app.bin,0755
files/app.conf,/etc/app/app.conf,
```

- `local_path` göreli ise workspace klasörüne göre çözülür. Dosya yoksa çalıştırma başlamaz.
- `remote_path` mutlak olmalı; tekrar eden hedef → hata.
- `mode` boşsa: güncellemede mevcut izin korunur, yeni dosyada `0644`.

### settings.json

```json
{
  "parallel": 10,
  "backup": true,
  "post_command": "",
  "post_command_policy": "on_change",
  "connect_timeout_sec": 10,
  "command_timeout_sec": 30,
  "strict_host_key": false
}
```

- `post_command_policy`: `on_change` (en az bir dosya CREATED/UPDATED ise), `always`, `never`.

## 6. Bağlantı ve protokol seçimi

1. SSH (port 22 veya CSV'deki port) ile bağlanmayı dene (şifre + keyboard-interactive).
2. TCP bağlantısı reddedilir/zaman aşımı olursa Telnet (23) dene.
3. SSH'a bağlanıldı ama kimlik reddedildiyse yine Telnet denenir; ikisi de başarısızsa cihaz `FAILED`, sebep her iki denemenin hatasını içerir.
4. Bağlantı hatasında bir kez yeniden denenir.

SSH host key: varsayılan kabul edilir. `strict_host_key: true` ise workspace'teki `known_hosts` ile TOFU (ilk görüşte kaydet, sonra uyuşmazlıkta reddet).

### Shell/marker modu (Telnet ve exec'siz SSH)

- Login sonrası prompt beklenir, sonra `stty -echo` denenir (başarısız olursa echo satırları filtrelenir).
- Her komut şu şekilde gönderilir: `cmd; echo "__DU_<nonce>_$?__"`; çıktı marker görülene kadar okunur, exit code marker'dan alınır.
- Satır uzunluğu sınırlarına karşı tek komut satırı ~1 KB'yi geçmez.

## 7. Probe (yetenek tespiti)

Bağlantı sonrası tek komutla yoklanır (`command -v` ya da `which`, ikisi de yoksa `type`):

`md5sum sha256sum base64 od hexdump printf stat mkdir mv cp chmod rm cat`

SSH'da ayrıca SFTP alt sistemi ve `scp` varlığı; hosttan cihazın 21 portuna TCP ile FTP varlığı kontrol edilir.

Seçim:

- **Yükleme yöntemi (ilk uygun):** SFTP → SCP → FTP → shell-base64 (`base64 -d` varsa) → shell-printf (`printf` octal escape ile, ash builtin).
- **Hash yöntemi:** `sha256sum` → `md5sum` → geri okuma (base64 → `od -An -tx1` → `hexdump`) ile hostta karşılaştırma. Hiçbiri yoksa karşılaştırma yapılamaz: dosya her zaman `UPDATED` olarak yazılır ve raporda "karşılaştırılamadı" notu düşülür.
- **Mode okuma:** `stat -c %a` → `ls -ln` çıktısını parse.

Probe sonucu raporda ve UI bağlantı testinde gösterilir.

## 8. Senkronizasyon akışı (cihaz başına)

1. Bağlan (bölüm 6). Başarısızsa manifest'teki tüm dosyalar `FAILED`.
2. Probe (bölüm 7).
3. Her manifest satırı için:
   1. Yerel hash — çalıştırma başında bir kez hesaplanır, tüm cihazlarda paylaşılır.
   2. Uzak dosya var mı (`[ -f path ]`)? Yoksa → `CREATE`. Varsa hash karşılaştır → eşit `UNCHANGED`, farklı `UPDATE`.
   3. Dry-run ise dur: `WOULD_CREATE` / `WOULD_UPDATE` / `UNCHANGED`.
   4. `mkdir -p` üst dizin.
   5. `<remote>.devupd.tmp` dosyasına yükle.
   6. Tmp dosyanın hash'ini doğrula (hash yöntemi varsa).
   7. `backup` açıksa ve dosya varsa: `cp -p remote remote.bak` (her seferinde üzerine yazılır).
   8. `chmod` (manifest mode, yoksa eski mode, yeni dosyada 0644).
   9. `mv tmp remote` (aynı dizinde atomik).
   10. Herhangi bir adımda hata: tmp silinir, orijinal dosyaya dokunulmaz, dosya `FAILED` + sebep, sonraki dosyaya geçilir.
4. `post_command` politikaya göre çalıştırılır; stdout/stderr (kırpılmış) ve exit code rapora yazılır.

Durumlar: `UNCHANGED`, `CREATED`, `UPDATED`, `WOULD_CREATE`, `WOULD_UPDATE`, `FAILED`.

## 9. Runner

- Worker pool, `parallel` kadar cihaz aynı anda. Varsayılan 10.
- Cihaz başına panic/hata sadece o cihazı etkiler.
- `context` ile iptal: aktif transferler kesilir, tmp temizlenmeye çalışılır, hedef dosya atomik mv sayesinde bozulmaz. İptal edilen cihazlar raporda `FAILED (cancelled)`.
- Aynı anda tek çalıştırma (UI'da ikinci başlatma reddedilir).
- Event tipleri: `device_state` (connecting, probing, syncing n/m, post_command, done, failed), `file_result`, `run_done`.

## 10. Rapor

- Her çalıştırmada `reports/YYYYMMDD-HHMMSS.html` (tek dosya, inline CSS, dışa bağımlılıksız) ve aynı isimli `.json`.
- İçerik: çalıştırma parametreleri (dry-run, paralel, manifest özeti), özet sayılar, cihaz başına protokol/yükleme yöntemi/süre/post_command sonucu, cihaz × dosya satırları (durum, yöntem, süre, hata).
- Şifreler rapor, log ve JSON'a asla yazılmaz.
- CLI sonunda konsol özeti: cihaz ve dosya bazında durum sayıları + başarısız cihaz listesi.
- CLI exit code: `0` başarısız yok, `1` en az bir `FAILED`, `2` yapılandırma/doğrulama hatası.

## 11. Web UI

`devupdater ui` (veya binary'ye çift tıklama): `127.0.0.1` üzerinde rastgele port, rastgele erişim token'ı ile açılır, varsayılan tarayıcı otomatik açılır. Token olmadan API istekleri reddedilir.

Üstte stepper bulunan sihirbaz. Her adım doğrulanmadan sonrakine geçilmez, geri dönülebilir, girilen her şey workspace'e kaydedilir.

1. **Çalışma alanı** — yeni oluştur / aç / son kullanılanlar. Klasör seçimi sunucu tarafı dosya gezgini ile (tarayıcı gerçek path vermediği için).
2. **Cihazlar** — tablo (ekle, düzenle, sil, toplu yapıştır), CSV içe/dışa aktar, şifreler maskeli. **Bağlantı testi** (seçili/tüm): satırda protokol, bulunan araçlar, seçilecek yükleme ve hash yöntemi. Erişilemeyen cihaz uyarıyla işaretlenir; operatör çıkarabilir veya devam edebilir.
3. **Dosyalar** — sürükle-bırak veya gezgin ile dosya ekleme (dosya workspace `files/` altına kopyalanır). Satır başına hedef path ve mode; anlık doğrulama.
4. **Ayarlar** — paralellik, yedek, post_command + politika; timeout'lar ve strict host key "Gelişmiş" bölümünde.
5. **Önizleme (dry-run)** — zorunlu adım. Cihaz × dosya matrisi (yeni / değişecek / aynı / erişilemez) ve özet cümlesi. Operatör açıkça onaylar.
6. **Uygula** — canlı grid (SSE): cihaz başına aşama ve dosya ilerlemesi, hatalar satırda. İptal butonu.
7. **Rapor** — özet kartları + detay tablo, "Başarısızları tekrar dene" (sadece FAILED cihazlarla yeni çalıştırma), HTML raporu aç/indir.

Ayrıca menüden **Geçmiş raporlar** listesi.

Görsel tasarım (renk, tipografi, bileşen detayları) implementasyon aşamasında frontend tasarım skill'leriyle belirlenir.

### API (özet)

- `GET/POST /api/workspace` (aç, oluştur, son kullanılanlar), `GET /api/fs?path=` (gezgin)
- `GET/PUT /api/devices`, `POST /api/devices/import`, `GET /api/devices/export`
- `GET/PUT /api/manifest`, `POST /api/files` (upload → `files/`)
- `GET/PUT /api/settings`
- `POST /api/test-connection` (seçili cihazlar, SSE ile sonuç)
- `POST /api/runs` (`dry_run`, `only_failed_from`), `DELETE /api/runs/current` (iptal), `GET /api/runs/current/events` (SSE)
- `GET /api/reports`, `GET /api/reports/{id}` (HTML/JSON)

## 12. Test stratejisi

- **Unit:** CSV/manifest/settings parse ve doğrulama; karar mantığı ve atomik yazma sırası (sahte Session/Uploader); shell-base64 ve shell-printf encoder round-trip (binary içerik dahil); marker parser; telnet istemcisi (in-process sahte telnet sunucusu); rapor üretimi (şifre sızmaması kontrolü).
- **Entegrasyon (Docker, busybox tabanlı cihaz matrisi):**
  - A: SSH + SFTP
  - B: SSH (dropbear), scp/sftp yok
  - C: Telnet + ftpd
  - D: Telnet, sadece sh/echo/cat/printf (base64, md5sum yok)
  - Her senaryoda: create, update, unchanged, dry-run, mode, backup, post_command, yetkisiz path → FAILED.
- **UI:** bileşen testleri (Vitest) + Playwright ile sihirbazın uçtan uca akışı (sahte cihazlara karşı).

## 13. Build ve dağıtım

- `make release`: frontend build → Go cross-compile tüm hedefler → `dist/devupdater-<os>-<arch>[.exe]`.
- Çalıştırmak için hostta hiçbir runtime gerekmez.
