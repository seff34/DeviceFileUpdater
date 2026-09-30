import { ArrowClockwise, ArrowSquareOut, CaretRight, CheckCircle, DownloadSimple, XCircle } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Fragment, memo, useCallback, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { Button } from '@/components/ui/button'
import { api, ApiError, reportHtmlUrl } from '@/lib/api'
import { formatDateTime, formatDuration } from '@/lib/format'
import { navigate } from '@/lib/router'
import { deviceFailed } from '@/lib/status'
import { CANCELLED_TEXT, fileErrorText, hasRealError, isCancelled, resultSentence, tally } from '@/lib/summary'
import type { DeviceResult, RunResult } from '@/lib/types'
import { cn } from '@/lib/utils'
import { useWizard } from '@/wizard/WizardContext'

const RETRY_LIST_MAX = 10

function Stat({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className="px-5 py-4">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={cn('mt-1 text-2xl font-semibold tabular-nums tracking-tight', tone)}>{value}</p>
    </div>
  )
}

// The server's messages are already Turkish and say which cause applies (409 and 422 have several).
function retryMessage(e: unknown): string {
  const msg = (e as Error).message
  if (msg) return msg
  if (e instanceof ApiError) {
    if (e.status === 409) return 'Şu anda çalışan bir işlem var. Bitmesini bekleyip tekrar deneyin.'
    if (e.status === 422) return 'Tekrar denenecek başarısız cihaz yok.'
    if (e.status === 404) return 'Rapor bulunamadı.'
  }
  return 'Tekrar deneme başlatılamadı.'
}

const DeviceRow = memo(function DeviceRow({ d, expanded, onToggle }: { d: DeviceResult; expanded: boolean; onToggle: (host: string) => void }) {
  const failed = deviceFailed(d)
  const cancelled = isCancelled(d)
  return (
    <Fragment>
      <tr>
        <td className="px-3 py-2">
          <button type="button" onClick={() => onToggle(d.host)} aria-expanded={expanded} className="flex items-center gap-1.5 rounded-md font-mono outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <CaretRight size={12} className={cn('transition-transform duration-150 ease-[var(--ease-out-strong)] motion-reduce:transition-none', expanded && 'rotate-90')} aria-hidden />
            {d.host}
          </button>
        </td>
        <td className="px-3 py-2">
          <span className={cn('inline-flex items-center gap-1.5', failed ? 'text-fail' : 'text-ok')}>
            {failed ? <XCircle size={16} weight="bold" aria-hidden /> : <CheckCircle size={16} weight="bold" aria-hidden />}
            {cancelled ? CANCELLED_TEXT : failed ? 'Başarısız' : 'Başarılı'}
          </span>
          {hasRealError(d) && <p className="mt-0.5 max-w-80 truncate text-xs text-fail" title={d.error}>{d.error}</p>}
        </td>
        <td className="px-3 py-2 font-mono text-xs text-muted-foreground">{[d.protocol, d.upload_method, d.hash_method].filter(Boolean).join(' · ') || '-'}</td>
        <td className="px-3 py-2 tabular-nums text-muted-foreground">{formatDuration(d.duration_ms)}</td>
        <td className="px-3 py-2">
          {d.post ? (
            <span className={cn('font-mono text-xs', d.post.exit_code !== 0 || d.post.error ? 'text-fail' : 'text-muted-foreground')}>çıkış {d.post.exit_code}</span>
          ) : (
            <span className="text-muted-foreground">-</span>
          )}
        </td>
      </tr>
      {expanded && (
        <tr className="bg-muted/40">
          <td colSpan={5} className="px-3 py-3">
            {d.files.length === 0 && <p className="text-xs text-muted-foreground">Bu cihaz için dosya sonucu yok.</p>}
            <ul className="grid gap-1.5">
              {d.files.map((f) => (
                <li key={f.remote} className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3 md:grid-cols-[minmax(0,2fr)_9rem_8rem_5rem_minmax(0,2fr)]">
                  <span className="truncate font-mono text-xs" title={f.remote}>{f.remote}</span>
                  <StatusBadge status={f.status} />
                  <span className="hidden font-mono text-xs text-muted-foreground md:block">{f.method ?? ''}</span>
                  <span className="hidden text-xs tabular-nums text-muted-foreground md:block">{formatDuration(f.duration_ms)}</span>
                  <span className="col-span-2 break-words text-xs text-fail md:col-span-1">{fileErrorText(f.error)}</span>
                </li>
              ))}
            </ul>
            {d.post && (
              <div className="mt-3">
                <p className="text-xs text-muted-foreground">
                  <span className="font-mono">{d.post.command}</span> · çıkış kodu {d.post.exit_code}
                  {d.post.error && <span className="text-fail"> · {d.post.error}</span>}
                </p>
                {d.post.output && <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap rounded-md border bg-card p-2 font-mono text-xs">{d.post.output}</pre>}
              </div>
            )}
          </td>
        </tr>
      )}
    </Fragment>
  )
})

export function ReportView({ result, reportId, allowRetry }: { result: RunResult; reportId: string; allowRetry: boolean }) {
  const qc = useQueryClient()
  const { facts } = useWizard()
  const t = tally(result)
  const b = t.byStatus
  const failedHosts = useMemo(() => result.devices.filter(deviceFailed).map((d) => d.host), [result])
  const [onlyFailed, setOnlyFailed] = useState(false)
  const [open, setOpen] = useState<Set<string>>(new Set())
  const [confirmRetry, setConfirmRetry] = useState(false)
  const devices = useQuery({ queryKey: ['devices'], queryFn: api.devices, enabled: confirmRetry })
  const busy = facts.run?.state === 'running'
  const retry = useMutation({
    mutationFn: () => api.startRun({ dry_run: false, only_failed_from: reportId }),
    onSuccess: (st) => {
      qc.setQueryData(['run'], st)
      if (st.total_devices) toast(`${st.total_devices} cihazda tekrar deneme başladı.`)
      navigate('/apply')
    },
    onError: (e) => toast.error(retryMessage(e)),
  })
  const rows = useMemo(() => result.devices.filter((d) => !onlyFailed || deviceFailed(d)), [result, onlyFailed])
  const toggle = useCallback((h: string) => setOpen((s) => { const x = new Set(s); if (x.has(h)) x.delete(h); else x.add(h); return x }), [])
  const elapsed = result.started && result.finished ? new Date(result.finished).getTime() - new Date(result.started).getTime() : 0
  // Only failed hosts still in devices.csv are retried by the server.
  const current = useMemo(() => new Set((devices.data ?? []).map((d) => d.host)), [devices.data])
  const retryHosts = failedHosts.filter((h) => current.has(h))
  const skipped = failedHosts.length - retryHosts.length
  const loading = devices.isPending
  const shown = retryHosts.slice(0, RETRY_LIST_MAX)
  const confirmBlock = loading ? 'Cihaz listesi yükleniyor.' : devices.isError ? 'Cihaz listesi okunamadı.' : retryHosts.length === 0 ? 'Başarısız cihazların hiçbiri artık devices.csv içinde değil.' : undefined

  return (
    <div className="grid grid-cols-1 gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-base">{resultSentence(result)}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {result.dry_run && <span className="mr-2 rounded-md border px-1.5 py-0.5 text-xs">Önizleme</span>}
            <span className="font-mono">{reportId}</span>
            {result.started && <> · {formatDateTime(result.started)}</>}
            {elapsed > 0 && <> · {formatDuration(elapsed)}</>} · paralel {result.parallel}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {allowRetry && !result.dry_run && failedHosts.length > 0 && (
            <Button onClick={() => setConfirmRetry(true)} disabled={busy || retry.isPending}>
              <ArrowClockwise aria-hidden /> Başarısızları tekrar dene
            </Button>
          )}
          <Button variant="outline" asChild>
            <a href={reportHtmlUrl(reportId)} target="_blank" rel="noopener"><ArrowSquareOut aria-hidden /> HTML raporu aç</a>
          </Button>
          <Button variant="outline" asChild>
            <a href={reportHtmlUrl(reportId, true)}><DownloadSimple aria-hidden /> İndir</a>
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-2 divide-x divide-y rounded-md border bg-card sm:grid-cols-3 lg:grid-cols-6 lg:divide-y-0">
        <Stat label="Cihaz" value={t.devices} />
        <Stat label="Başarısız cihaz" value={t.failedDevices} tone={t.failedDevices ? 'text-fail' : undefined} />
        {result.dry_run ? (
          <>
            <Stat label="Oluşturulacak dosya" value={b.WOULD_CREATE ?? 0} />
            <Stat label="Güncellenecek dosya" value={b.WOULD_UPDATE ?? 0} />
          </>
        ) : (
          <>
            <Stat label="Oluşturulan dosya" value={b.CREATED ?? 0} />
            <Stat label="Güncellenen dosya" value={b.UPDATED ?? 0} />
          </>
        )}
        <Stat label="Aynı kalan dosya" value={b.UNCHANGED ?? 0} />
        <Stat label="Başarısız dosya" value={b.FAILED ?? 0} tone={b.FAILED ? 'text-fail' : undefined} />
      </div>

      <div>
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-sm font-medium">Cihazlar</h2>
          <Button size="sm" variant={onlyFailed ? 'secondary' : 'ghost'} aria-pressed={onlyFailed} onClick={() => setOnlyFailed((v) => !v)}>
            Sadece başarısızlar <span className="tabular-nums text-muted-foreground">{t.failedDevices}</span>
          </Button>
        </div>
        <div className="overflow-x-auto rounded-md border bg-card">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th scope="col" className="px-3 py-2 font-medium">Cihaz</th>
                <th scope="col" className="px-3 py-2 font-medium">Durum</th>
                <th scope="col" className="px-3 py-2 font-medium">Bağlantı</th>
                <th scope="col" className="px-3 py-2 font-medium">Süre</th>
                <th scope="col" className="px-3 py-2 font-medium">post-command</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {rows.map((d) => <DeviceRow key={d.host} d={d} expanded={open.has(d.host)} onToggle={toggle} />)}
            </tbody>
          </table>
          {rows.length === 0 && <p className="p-6 text-sm text-muted-foreground">Gösterilecek cihaz yok.</p>}
        </div>
      </div>

      <ConfirmDialog
        open={confirmRetry}
        onOpenChange={setConfirmRetry}
        title="Başarısız cihazlar tekrar denensin mi?"
        description={
          confirmBlock ? (
            <>{confirmBlock}</>
          ) : (
            <>
              Bu işlem önizlemesiz, doğrudan cihazlara yazar. Yalnızca {retryHosts.length} cihazda yeni bir uygulama başlatılır: <span className="font-mono">{shown.join(', ')}</span>
              {retryHosts.length > shown.length && <> ve {retryHosts.length - shown.length} cihaz daha</>}.
              {skipped > 0 && <> {skipped} cihaz artık devices.csv'de olmadığı için atlanacak.</>} Tekrar deneme, bu rapordakinden farklı olabilecek güncel dosya listesini ve ayarları kullanır.
            </>
          )
        }
        confirmLabel="Tekrar dene"
        confirmDisabled={!!confirmBlock}
        onConfirm={() => retry.mutate()}
      />
    </div>
  )
}
