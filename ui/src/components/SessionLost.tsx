import { PlugsConnected } from '@phosphor-icons/react'

export function SessionLost() {
  return (
    <main className="flex min-h-dvh items-center justify-center p-6">
      <div className="max-w-md text-center">
        <PlugsConnected size={32} className="mx-auto text-muted-foreground" aria-hidden />
        <h1 className="mt-4 text-lg font-semibold">Oturum bulunamadı</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Bu sayfa güvenlik anahtarı olmadan açıldı veya uygulama yeniden başlatıldı. Uygulamayı kapatıp
          <code className="mx-1 rounded bg-muted px-1 font-mono">devupdater ui</code>
          ile yeniden açın ya da terminalde yazan adresi kullanın.
        </p>
      </div>
    </main>
  )
}
