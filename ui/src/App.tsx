import { useEffect, type JSX } from 'react'
import { AppShell } from '@/components/AppShell'
import { SessionLost } from '@/components/SessionLost'
import { Stepper } from '@/components/Stepper'
import { navigate, reportIdFromPath, usePath } from '@/lib/router'
import { useSessionLost } from '@/lib/session'
import { STEPS, blocker, canEnter, stepByPath, type StepId } from '@/lib/steps'
import { Help } from '@/pages/Help'
import { History, ReportPage } from '@/pages/History'
import { ApplyStep } from '@/steps/ApplyStep'
import { DevicesStep } from '@/steps/DevicesStep'
import { FilesStep } from '@/steps/FilesStep'
import { PreviewStep } from '@/steps/PreviewStep'
import { ReportStep } from '@/steps/ReportStep'
import { SettingsStep } from '@/steps/SettingsStep'
import { WorkspaceStep } from '@/steps/WorkspaceStep'
import { useWizard } from '@/wizard/WizardContext'

const VIEWS: Record<StepId, () => JSX.Element> = {
  workspace: WorkspaceStep,
  devices: DevicesStep,
  files: FilesStep,
  settings: SettingsStep,
  preview: PreviewStep,
  apply: ApplyStep,
  report: ReportStep,
}

/**
 * First step the operator should be on: a running real run wins, a finished
 * real run opens its report, else the first blocked step.
 */
function landing(f: ReturnType<typeof useWizard>['facts']): string {
  if (f.run?.state === 'running' && !f.run.dry_run) return '/apply'
  if (f.run?.state === 'done' && !f.run.dry_run) return '/report'
  const first = STEPS.find((s) => blocker(s.id, f) !== null)
  return (first ?? STEPS[STEPS.length - 1]).path
}

export default function App() {
  const lost = useSessionLost()
  const path = usePath()
  const { facts, loading } = useWizard()
  const step = stepByPath(path)
  const reportId = reportIdFromPath(path)

  useEffect(() => {
    if (loading || lost) return
    if (path === '/' || (!step && !reportId && path !== '/history' && path !== '/help')) {
      navigate(landing(facts), { replace: true })
    } else if (step && !canEnter(step.id, facts)) {
      navigate(landing(facts), { replace: true })
    }
  }, [path, step, reportId, facts, loading, lost])

  if (lost) return <SessionLost />
  if (reportId) return <AppShell><ReportPage id={reportId} /></AppShell>
  if (path === '/history') return <AppShell><History /></AppShell>
  if (path === '/help') return <AppShell><Help /></AppShell>
  if (!step) return <AppShell>{null}</AppShell>
  const View = VIEWS[step.id]
  return (
    <AppShell stepper={<Stepper current={step.id} />}>
      <View />
    </AppShell>
  )
}
