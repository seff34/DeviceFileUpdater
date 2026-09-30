import { ArrowClockwise, Play, Stop, WarningCircle } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { PreviewMatrix } from '@/components/PreviewMatrix'
import { RunProgress } from '@/components/RunProgress'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { navigate } from '@/lib/router'
import { blocker } from '@/lib/steps'
import { filterDevices, previewSentence, tally, type MatrixFilter } from '@/lib/summary'
import { useRunEvents } from '@/lib/useRunEvents'
import { cn } from '@/lib/utils'
import { useWizard } from '@/wizard/WizardContext'

const FILTERS: { id: MatrixFilter; label: string }[] = [
  { id: 'all', label: 'Tümü' },
  { id: 'changes', label: 'Değişecek' },
  { id: 'failed', label: 'Hatalı' },
]

function confirmText(t: ReturnType<typeof tally>): string {
  const head = t.changedDevices
    ? `Değişiklikleri inceledim. ${t.changedDevices} cihazda toplam ${t.filesCreate + t.filesUpdate} dosya yazılacak`
    : 'Değişiklikleri inceledim. Hiçbir cihaza dosya yazılmayacak'
  return t.unreachable ? `${head}; ulaşılamayan ${t.unreachable} cihaz raporda başarısız görünecek.` : `${head}.`
}

export function PreviewStep() {
  const qc = useQueryClient()
  const { facts, previewConfirmed, setPreviewConfirmed } = useWizard()
  const run = facts.run
  const devices = useQuery({ queryKey: ['devices'], queryFn: api.devices })
  const hosts = (devices.data ?? []).map((d) => d.host)
  const [filter, setFilter] = useState<MatrixFilter>('all')

  const previewRunning = run?.state === 'running' && run.dry_run
  const live = useRunEvents(previewRunning && devices.isSuccess, hosts)
  const previewId = !previewRunning ? run?.preview_id || null : null
  const report = useQuery({ queryKey: ['report', previewId], queryFn: () => api.report(previewId!), enabled: !!previewId })
  const fresh = !!run?.preview_fresh

  // A new preview starts from the unfiltered view (render-time adjust, not an effect).
  const [seenPreview, setSeenPreview] = useState(previewId)
  if (seenPreview !== previewId) {
    setSeenPreview(previewId)
    setFilter('all')
  }

  const start = useMutation({
    mutationFn: () => api.startRun({ dry_run: true }),
    onSuccess: (st) => qc.setQueryData(['run'], st),
    onError: (e) => toast.error((e as Error).message),
  })
  const stop = useMutation({ mutationFn: api.cancelRun, onSuccess: () => qc.invalidateQueries({ queryKey: ['run'] }) })
  const apply = useMutation({
    mutationFn: () => api.startRun({ dry_run: false, preview_id: run!.preview_id }),
    onSuccess: (st) => {
      qc.setQueryData(['run'], st)
      navigate('/apply')
    },
    onError: (e) => {
      toast.error((e as Error).message)
      void qc.invalidateQueries({ queryKey: ['run'] })
    },
  })

  const ready = devices.isSuccess
  const gate = devices.isPending ? 'Cihazlar yükleniyor.' : !ready ? 'Cihazlar yüklenemedi.' : blocker('preview', facts)
  const t = report.data ? tally(report.data) : null
  const cancelledPreview = run?.state === 'done' && run.dry_run && !run.preview_id
  // The last dry run failed or was cancelled, so the matrix below is an older preview.
  const lastRunIncomplete = run?.state === 'done' && run.dry_run && !!run.preview_id && (!!run.error || run.report_id !== run.preview_id)

  return (
    <StepPage
      step="preview"
      action={{
        onBack: () => navigate('/settings'),
        primaryLabel: 'Uygulamayı başlat',
        onPrimary: () => apply.mutate(),
        primaryBusy: apply.isPending,
        blocker: gate,
      }}
      aside={
        previewRunning ? (
          <Button variant="outline" onClick={() => stop.mutate()} disabled={stop.isPending}>
            <Stop aria-hidden /> Durdur
          </Button>
        ) : (
          <Button variant={previewId ? 'outline' : 'default'} onClick={() => start.mutate()} disabled={!ready || start.isPending}>
            {previewId ? <ArrowClockwise aria-hidden /> : <Play aria-hidden />}
            {previewId ? 'Önizlemeyi yeniden çalıştır' : 'Önizlemeyi başlat'}
          </Button>
        )
      }
    >
      {run?.error && (
        <Alert variant="destructive" className="mb-4"><AlertDescription>{run.error}</AlertDescription></Alert>
      )}
      {devices.isPending ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-10" />)}</div>
      ) : !ready ? (
        <Alert variant="destructive"><AlertDescription>{(devices.error as Error | null)?.message ?? 'Cihazlar yüklenemedi.'}</AlertDescription></Alert>
      ) : previewRunning ? (
        <div className="grid gap-4">
          <RunProgress view={live} total={run?.total_devices ?? hosts.length} />
          <p className="text-sm text-muted-foreground">Cihazlara bağlanılıyor ve dosyalar karşılaştırılıyor. Önizleme cihazlarda hiçbir şeyi değiştirmez.</p>
        </div>
      ) : !previewId ? (
        <div className="rounded-md border bg-card p-8">
          {cancelledPreview && <p className="mb-2 text-sm text-warn">Önceki önizleme durduruldu.</p>}
          <p className="font-medium">Henüz önizleme yok</p>
          <p className="mt-1 max-w-[65ch] text-sm text-muted-foreground">
            Önizleme (dry-run) her cihaza bağlanır, dosyaları karşılaştırır ve neyin oluşturulacağını, güncelleneceğini veya aynı kalacağını gösterir. Cihazlara hiçbir şey yazılmaz. Uygulamadan önce zorunludur.
          </p>
        </div>
      ) : report.isPending ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-10" />)}</div>
      ) : report.isError ? (
        <Alert variant="destructive"><AlertDescription>{(report.error as Error).message}</AlertDescription></Alert>
      ) : (
        <div className="grid gap-4">
          {lastRunIncomplete && (
            <Alert className="border-warn/30 bg-warn/5">
              <WarningCircle className="text-warn" aria-hidden />
              <AlertDescription className="text-foreground">Son önizleme tamamlanmadı; gösterilen sonuç önceki önizlemeye ait.</AlertDescription>
            </Alert>
          )}
          {!fresh && (
            <Alert className="border-warn/30 bg-warn/5">
              <WarningCircle className="text-warn" aria-hidden />
              <AlertDescription className="text-foreground">
                Önizlemeden sonra cihazlar, dosyalar veya ayarlar değişti. Uygulamadan önce önizlemeyi yeniden çalıştırın.
              </AlertDescription>
            </Alert>
          )}
          <p className="text-base">{previewSentence(report.data)}</p>
          <div className="flex flex-wrap items-center gap-1" role="group" aria-label="Filtre">
            {FILTERS.map((f) => {
              const n = filterDevices(report.data, f.id).length
              return (
                <Button
                  key={f.id}
                  size="sm"
                  variant={filter === f.id ? 'secondary' : 'ghost'}
                  aria-pressed={filter === f.id}
                  onClick={() => setFilter(f.id)}
                  className={cn(filter === f.id && 'font-medium')}
                >
                  {f.label} <span className="tabular-nums text-muted-foreground">{n}</span>
                </Button>
              )
            })}
          </div>
          <PreviewMatrix result={report.data} filter={filter} />
          <div className={cn('flex items-start gap-3 rounded-md border bg-card p-4', !fresh && 'opacity-60')}>
            <Checkbox id="preview-ok" checked={previewConfirmed} disabled={!fresh} onCheckedChange={(v) => setPreviewConfirmed(v === true)} className="mt-0.5" />
            <Label htmlFor="preview-ok" className="text-sm leading-relaxed font-normal">
              {confirmText(t!)}
            </Label>
          </div>
        </div>
      )}
    </StepPage>
  )
}
