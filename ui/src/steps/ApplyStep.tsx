import { StopCircle } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { RunGrid, type GridFilter } from '@/components/RunGrid'
import { RunProgress } from '@/components/RunProgress'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { api, ApiError } from '@/lib/api'
import { navigate } from '@/lib/router'
import { runCounts } from '@/lib/runState'
import { deviceFailed } from '@/lib/status'
import { blocker } from '@/lib/steps'
import { resultSentence } from '@/lib/summary'
import { useRunEvents } from '@/lib/useRunEvents'
import { useWizard } from '@/wizard/WizardContext'

export function ApplyStep() {
  const qc = useQueryClient()
  const { facts } = useWizard()
  const run = facts.run
  // A dry run (preview, or a dry run of "only failed") is never shown here.
  const real = !!run && !run.dry_run && run.state !== 'idle'
  const only = run?.only_failed_from ?? ''
  const devices = useQuery({ queryKey: ['devices'], queryFn: api.devices, enabled: real && !only })
  const prev = useQuery({ queryKey: ['report', only], queryFn: () => api.report(only), enabled: real && !!only })
  const source = only ? prev : devices
  const hosts = only ? (prev.data?.devices.filter(deviceFailed).map((d) => d.host) ?? []) : (devices.data ?? []).map((d) => d.host)
  const ready = real && source.isSuccess
  const view = useRunEvents(ready, hosts)
  const running = run?.state === 'running'
  const [confirmCancel, setConfirmCancel] = useState(false)
  const [filter, setFilter] = useState<GridFilter>('all')
  const cancel = useMutation({
    mutationFn: api.cancelRun,
    onSuccess: () => toast('İptal ediliyor. Süren aktarımlar kesiliyor.'),
    onError: (e) => {
      // 404 means no run is active any more: it finished while the dialog was open.
      if (e instanceof ApiError && e.status === 404) void qc.invalidateQueries({ queryKey: ['run'] })
      else toast.error((e as Error).message)
    },
  })
  const failed = view ? runCounts(view).failed : 0
  const gate = running && source.isPending ? 'Cihazlar yükleniyor.' : blocker('apply', facts)

  return (
    <StepPage
      step="apply"
      action={{
        primaryLabel: 'Raporu gör',
        onPrimary: () => navigate('/report'),
        blocker: gate,
        extra: running ? (
          <Button variant="outline" className="text-fail hover:text-fail" onClick={() => setConfirmCancel(true)} disabled={cancel.isPending || cancel.isSuccess}>
            <StopCircle aria-hidden /> İptal et
          </Button>
        ) : null,
      }}
      aside={
        only ? <p className="text-sm text-muted-foreground">Sadece <span className="font-mono">{only}</span> raporundaki başarısız cihazlar</p> : undefined
      }
    >
      {real && run.error && (
        <Alert variant="destructive" className="mb-4"><AlertDescription>{run.error}</AlertDescription></Alert>
      )}
      {!real ? (
        <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">Şu anda çalışan bir uygulama yok.</p>
      ) : source.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{(source.error as Error).message || 'Cihazlar yüklenemedi.'}</AlertDescription>
        </Alert>
      ) : !view ? (
        <div className="space-y-2">{[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-11" />)}</div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {view.result ? (
            <p className="text-base" role="status">{resultSentence(view.result)}</p>
          ) : (
            <RunProgress view={view} total={run.total_devices} />
          )}
          <div className="flex items-center gap-1" role="group" aria-label="Filtre">
            <Button size="sm" variant={filter === 'all' ? 'secondary' : 'ghost'} aria-pressed={filter === 'all'} onClick={() => setFilter('all')}>
              Tümü <span className="tabular-nums text-muted-foreground">{view.order.length}</span>
            </Button>
            <Button size="sm" variant={filter === 'failed' ? 'secondary' : 'ghost'} aria-pressed={filter === 'failed'} onClick={() => setFilter('failed')}>
              Başarısız <span className="tabular-nums text-muted-foreground">{failed}</span>
            </Button>
          </div>
          <RunGrid view={view} filter={filter} />
        </div>
      )}
      <ConfirmDialog
        open={confirmCancel && running}
        onOpenChange={setConfirmCancel}
        title="Çalıştırma iptal edilsin mi?"
        description="Süren aktarımlar kesilir. Cihazdaki asıl dosyalar bozulmaz, çünkü yükleme önce geçici dosyaya yapılır. Tamamlanmamış cihazlar raporda başarısız görünür."
        confirmLabel="Çalıştırmayı iptal et"
        destructive
        onConfirm={() => cancel.mutate()}
      />
    </StepPage>
  )
}
