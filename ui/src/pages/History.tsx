import { ArrowLeft, CaretRight } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { ReportView } from '@/components/ReportView'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { Link } from '@/lib/router'
import { STATUS_META } from '@/lib/status'
import type { Status } from '@/lib/types'

function Page({ title, back, children }: { title: string; back?: boolean; children: ReactNode }) {
  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
      {back && (
        <Link to="/history" className="mb-3 inline-flex items-center gap-1 rounded-md text-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring">
          <ArrowLeft size={14} aria-hidden /> Geçmiş raporlar
        </Link>
      )}
      <h1 className="mb-6 text-2xl font-semibold tracking-tight">{title}</h1>
      {children}
    </main>
  )
}

function useHasWorkspace() {
  const ws = useQuery({ queryKey: ['workspace'], queryFn: api.workspace })
  return { ready: !ws.isPending, has: !!ws.data?.current }
}

function NoWorkspace() {
  return (
    <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">
      Raporları görmek için önce bir çalışma alanı açın. <Link to="/workspace" className="text-primary underline-offset-4 hover:underline">Çalışma alanına git</Link>
    </p>
  )
}

const ORDER: Status[] = ['CREATED', 'UPDATED', 'WOULD_CREATE', 'WOULD_UPDATE', 'UNCHANGED', 'FAILED']

export function History() {
  const ws = useHasWorkspace()
  const q = useQuery({ queryKey: ['reports'], queryFn: api.reports, enabled: ws.has })
  return (
    <Page title="Geçmiş raporlar">
      {!ws.ready ? null : !ws.has ? (
        <NoWorkspace />
      ) : q.isPending ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-12" />)}</div>
      ) : q.isError ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error).message}</AlertDescription></Alert>
      ) : q.data.length === 0 ? (
        <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">Bu çalışma alanında henüz rapor yok. Her önizleme ve uygulama bir rapor oluşturur.</p>
      ) : (
        <ul className="divide-y rounded-md border bg-card">
          {q.data.map((r) => (
            <li key={r.id}>
              <Link to={`/history/${r.id}`} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1 px-4 py-3 text-sm outline-none hover:bg-muted focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring md:grid-cols-[12rem_7rem_9rem_minmax(0,1fr)_auto]">
                <span className="font-mono">{r.id}</span>
                <span className="text-muted-foreground">{r.dry_run ? 'Önizleme' : 'Uygulama'}</span>
                <span className="tabular-nums">
                  {r.devices} cihaz{r.devices_failed > 0 && <span className="text-fail"> · {r.devices_failed} başarısız</span>}
                </span>
                <span className="truncate text-muted-foreground">
                  {ORDER.filter((s) => r.by_status[s]).map((s) => `${r.by_status[s]} dosya ${STATUS_META[s].label.toLocaleLowerCase('tr-TR')}`).join(' · ')}
                </span>
                <span className="flex items-center gap-2 text-muted-foreground">
                  {r.started && formatDateTime(r.started)}
                  <CaretRight size={14} aria-hidden />
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Page>
  )
}

export function ReportPage({ id }: { id: string }) {
  const ws = useHasWorkspace()
  const q = useQuery({ queryKey: ['report', id], queryFn: () => api.report(id), enabled: ws.has })
  return (
    <Page title="Rapor" back>
      {!ws.ready ? null : !ws.has ? (
        <NoWorkspace />
      ) : q.isPending ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-16" />)}</div>
      ) : q.isError ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error).message}</AlertDescription></Alert>
      ) : (
        <ReportView result={q.data} reportId={id} allowRetry />
      )}
    </Page>
  )
}
