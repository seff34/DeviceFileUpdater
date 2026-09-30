import { ArrowRight } from '@phosphor-icons/react'
import type { ReactNode } from 'react'
import { Link } from '@/lib/router'
import { STEPS, type StepId } from '@/lib/steps'

const Mono = ({ children }: { children: ReactNode }) => <code className="rounded-md bg-muted px-1 py-0.5 font-mono text-[0.8125rem]">{children}</code>
const B = ({ children }: { children: ReactNode }) => <span className="font-medium text-foreground">{children}</span>

const GUIDE: Record<StepId, ReactNode[]> = {
  workspace: [
    <>Çalışma alanı, bir hat veya iş için her şeyin tutulduğu klasördür: <Mono>devices.csv</Mono>, <Mono>manifest.csv</Mono>, <Mono>settings.json</Mono>, <Mono>files/</Mono> ve <Mono>reports/</Mono>.</>,
    <>İlk kez: <B>Klasör seç</B> bölümünde bir üst klasöre gidin, <B>Yeni klasör adı</B> alanına bir ad yazın (örn. hat-3-guncelleme) ve <B>Oluştur ve kullan</B> deyin.</>,
    <>Var olan bir klasör için içine girip <B>Bu klasörü kullan</B> deyin; eksik dosyalar eklenir, var olanlara dokunulmaz. Daha önce açılanlar <B>Son kullanılanlar</B> listesinden tek tıkla açılır.</>,
  ],
  devices: [
    <>Cihazları <B>Cihaz ekle</B> ile tek tek girin veya <B>Toplu ekle</B> ile Excel'den IP, kullanıcı adı ve şifre sütunlarını yapıştırın.</>,
    <>Standart dışı port için IP'nin sonuna yazın: <Mono>10.0.0.21:2222</Mono>. SSH veya Telnet otomatik seçilir.</>,
    <><B>Tümünü test et</B> ile her cihaza bağlanın. Bağlantı sütunu protokolü ve bulunan araçları gösterir; kırmızı satırlar ulaşılamayan cihazlardır.</>,
    <>Ulaşılamayan cihazları <B>Ulaşılamayanları çıkar</B> ile listeden atabilir ya da bırakabilirsiniz; bırakılırsa raporda başarısız görünürler.</>,
  ],
  files: [
    <>Gönderilecek dosyaları alana sürükleyin veya seçin. Dosyalar çalışma alanındaki <Mono>files/</Mono> klasörüne kopyalanır.</>,
    <>Her dosya için <B>Cihazdaki hedef yol</B> tam yol olmalıdır (örn. <Mono>/etc/app/app.conf</Mono>). <Mono>/</Mono> ile biten bir klasör yazarsanız dosya adı eklenir.</>,
    <><B>İzin</B> sekizlik yazılır (örn. <Mono>0644</Mono>, çalıştırılabilir betik için <Mono>0755</Mono>). Boş bırakılırsa var olan dosyanın izni korunur.</>,
  ],
  settings: [
    <><B>Paralel cihaz sayısı</B> aynı anda kaç cihaza bağlanılacağını belirler (varsayılan 10). Ağ yavaşsa düşürün.</>,
    <><B>Yedek al</B> açıkken değişen dosyanın eski hali cihazda <Mono>&lt;hedef&gt;.bak</Mono> olarak saklanır.</>,
    <><B>post-command</B> dosyalardan sonra her cihazda çalışacak komuttur (örn. servisi yeniden başlatmak). Önce tek cihazla deneyin.</>,
  ],
  preview: [
    <><B>Önizlemeyi başlat</B> her cihaza bağlanır ve dosyaları karşılaştırır. Cihaza hiçbir şey yazılmaz.</>,
    <>Tabloda her cihaz ve dosya için sonuç görünür: <B>Oluşturulacak</B>, <B>Güncellenecek</B>, <B>Aynı</B> veya hata.</>,
    <>Sonucu inceleyip onay kutusunu işaretleyin, ardından <B>Uygulamayı başlat</B> deyin. Önizlemeden sonra cihaz, dosya veya ayar değişirse önizleme yeniden istenir.</>,
  ],
  apply: [
    <>Her cihazın aşaması ve dosya sayısı canlı güncellenir. <B>Başarısız</B> sekmesi yalnız sorunlu cihazları gösterir.</>,
    <>Gerekirse <B>İptal et</B> ile durdurun. Henüz bitmemiş cihazlar raporda iptal edildi olarak görünür.</>,
    <>Çalışma sürerken uygulamanın açık olduğu terminal penceresini kapatmayın.</>,
  ],
  report: [
    <>Özet, cihaz ve dosya sayılarını verir. Bir cihaz satırına tıklayınca dosya dosya sonuç ve hata açılır.</>,
    <><B>Başarısızları tekrar dene</B> yalnızca başarısız cihazlarla yeni bir uygulama başlatır.</>,
    <><B>HTML raporu aç</B> veya <B>İndir</B> ile raporu saklayın ya da iletin. Eski raporlar üstteki <B>Geçmiş raporlar</B> sayfasındadır.</>,
  ],
}

const TROUBLE: { q: string; a: ReactNode }[] = [
  { q: 'Cihaza ulaşılamadı', a: <>Bilgisayarın cihaz ağına bağlı olduğunu, IP ve portu kontrol edin. Hata metnindeki <Mono>connection refused</Mono> portun kapalı olduğunu, <Mono>timeout</Mono> cihaza ulaşılamadığını gösterir.</> },
  { q: 'Kullanıcı adı veya şifre hatası', a: <>Cihaz satırındaki bilgileri düzeltip <B>Tümünü test et</B> ile tekrar deneyin.</> },
  { q: '"Oturum bulunamadı" ekranı', a: <>Sayfa güvenlik anahtarı olmadan açıldı veya uygulama kapandı. Uygulamayı yeniden başlatın; tarayıcı doğru adresle kendiliğinden açılır.</> },
  { q: 'Uygulamayı kapatmak', a: <>Tarayıcı sekmesini kapatmak yetmez. Uygulamanın terminal penceresini kapatın veya içinde Ctrl+C'ye basın.</> },
]

export function Help() {
  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
      <h1 className="text-2xl font-semibold tracking-tight">Kullanım kılavuzu</h1>
      <p className="mt-1 max-w-[70ch] text-sm text-muted-foreground">
        Bir dosya setini cihazlara göndermek yedi adımdır. Üstteki adım çubuğu nerede olduğunuzu, alttaki çubuk bir sonraki adımı neyin engellediğini gösterir.
      </p>

      <ol className="mt-6 divide-y rounded-md border bg-card">
        {STEPS.map((s, i) => (
          <li key={s.id} className="grid grid-cols-1 gap-3 p-5 md:grid-cols-[14rem_minmax(0,1fr)] md:gap-6">
            <div>
              <h2 className="flex items-center gap-2 text-base font-medium">
                <span className="inline-grid size-6 shrink-0 place-items-center rounded-md border text-xs tabular-nums">{i + 1}</span>
                {s.title}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">{s.purpose}</p>
              <Link
                to={s.path}
                className="mt-2 inline-flex items-center gap-1 rounded-md text-sm text-primary underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
              >
                Bu adıma git <ArrowRight size={14} aria-hidden />
              </Link>
            </div>
            <ul className="grid list-disc gap-2 pl-5 text-sm text-muted-foreground marker:text-border">
              {GUIDE[s.id].map((t, j) => <li key={j} className="max-w-[75ch]">{t}</li>)}
            </ul>
          </li>
        ))}
      </ol>

      <h2 className="mt-10 text-base font-medium">Sık karşılaşılan durumlar</h2>
      <dl className="mt-3 divide-y rounded-md border bg-card">
        {TROUBLE.map((t) => (
          <div key={t.q} className="grid grid-cols-1 gap-1 p-5 md:grid-cols-[14rem_minmax(0,1fr)] md:gap-6">
            <dt className="text-sm font-medium">{t.q}</dt>
            <dd className="max-w-[75ch] text-sm text-muted-foreground">{t.a}</dd>
          </div>
        ))}
      </dl>
    </main>
  )
}
