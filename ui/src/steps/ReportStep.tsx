import { useQuery } from '@tanstack/react-query'
import { ReportView } from '@/components/ReportView'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import { Link, navigate } from '@/lib/router'
import { deviceFailed } from '@/lib/status'
import { useWizard } from '@/wizard/WizardContext'

export function ReportStep() {
  const { facts } = useWizard()
  const id = facts.run && !facts.run.dry_run ? (facts.run.report_id ?? '') : ''
  const q = useQuery({ queryKey: ['report', id], queryFn: () => api.report(id), enabled: !!id })
  // With failures, "Başarısızları tekrar dene" in the report is the primary action.
  const hasFailed = !!q.data?.devices.some(deviceFailed)
  return (
    <StepPage step="report" action={{ onBack: () => navigate('/apply'), primaryLabel: 'Yeni önizleme', onPrimary: () => navigate('/preview'), primaryVariant: hasFailed ? 'outline' : 'default' }}>
      {!id && facts.run?.state === 'running' ? (
        <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">
          Uygulama sürüyor. <Link to="/apply" className="text-primary underline-offset-4 hover:underline">İlerlemeyi gör</Link>
        </p>
      ) : !id ? (
        <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">Henüz tamamlanmış bir uygulama yok.</p>
      ) : q.isPending ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-16" />)}</div>
      ) : q.isError ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error).message}</AlertDescription></Alert>
      ) : (
        <ReportView result={q.data} reportId={id} allowRetry />
      )}
    </StepPage>
  )
}
