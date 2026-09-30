import { ClockCounterClockwise, FolderOpen } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { FolderBrowser } from '@/components/FolderBrowser'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import { navigate } from '@/lib/router'
import { blocker } from '@/lib/steps'
import { useWizard } from '@/wizard/WizardContext'

export function WorkspaceStep() {
  const qc = useQueryClient()
  const { facts } = useWizard()
  const ws = useQuery({ queryKey: ['workspace'], queryFn: api.workspace })
  const [browsing, setBrowsing] = useState(false)
  const open = useMutation({
    mutationFn: (v: { path: string; create: boolean }) => api.openWorkspace(v.path, v.create),
    onSuccess: async (st) => {
      qc.setQueryData(['workspace'], st)
      await qc.resetQueries({ predicate: (q) => q.queryKey[0] !== 'workspace' })
      setBrowsing(false)
      toast.success('Çalışma alanı açıldı')
    },
  })
  const current = ws.data?.current ?? ''
  const recent = (ws.data?.recent ?? []).filter((p) => p !== current)
  const showBrowser = !ws.isPending && !ws.isError && (!current || browsing)
  const start = (v: { path: string; create: boolean }) => {
    open.reset()
    open.mutate(v)
  }

  return (
    <StepPage step="workspace" action={{ onPrimary: () => navigate('/devices'), blocker: blocker('workspace', facts) }}>
      <div className="grid gap-8">
        {current && (
          <section aria-labelledby="ws-current" className="flex flex-wrap items-center gap-4 rounded-md border bg-card p-4">
            <FolderOpen size={24} className="text-primary" aria-hidden />
            <div className="min-w-0 flex-1">
              <h2 id="ws-current" className="text-sm font-medium">Açık çalışma alanı</h2>
              <p className="truncate font-mono text-sm text-muted-foreground" title={current}>{current}</p>
            </div>
            {!browsing && <Button variant="outline" onClick={() => { open.reset(); setBrowsing(true) }}>Başka klasör aç</Button>}
          </section>
        )}

        {ws.isPending && (
          <div className="space-y-2" aria-busy="true">
            <Skeleton className="h-16" />
            <Skeleton className="h-40" />
          </div>
        )}

        {ws.isError && (
          <Alert variant="destructive">
            <AlertDescription>{(ws.error as Error).message}</AlertDescription>
          </Alert>
        )}

        {open.isError && (
          <Alert variant="destructive">
            <AlertDescription>{(open.error as Error).message}</AlertDescription>
          </Alert>
        )}

        {recent.length > 0 && (
          <section aria-labelledby="ws-recent">
            <h2 id="ws-recent" className="mb-2 flex items-center gap-1.5 text-sm font-medium">
              <ClockCounterClockwise size={16} aria-hidden /> Son kullanılanlar
            </h2>
            <ul className="divide-y rounded-md border bg-card">
              {recent.map((p) => (
                <li key={p}>
                  <button
                    type="button"
                    disabled={open.isPending}
                    onClick={() => start({ path: p, create: false })}
                    className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm hover:bg-muted focus-visible:bg-muted outline-none disabled:opacity-60"
                  >
                    <FolderOpen size={16} className="shrink-0 text-muted-foreground" aria-hidden />
                    <span className="truncate font-mono">{p}</span>
                    <span className="ml-auto shrink-0 text-muted-foreground">Aç</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}

        {showBrowser && (
          <section aria-labelledby="ws-browse">
            <div className="mb-2 flex items-baseline justify-between gap-4">
              <h2 id="ws-browse" className="text-sm font-medium">Klasör seç</h2>
              <p className="text-xs text-muted-foreground">
                Seçtiğiniz klasöre eksikse <span className="font-mono">devices.csv</span>, <span className="font-mono">manifest.csv</span>,{' '}
                <span className="font-mono">settings.json</span>, <span className="font-mono">files/</span> ve <span className="font-mono">reports/</span> eklenir. Var olan dosyalara dokunulmaz.
              </p>
            </div>
            <FolderBrowser busy={open.isPending} onOpen={(path, create) => start({ path, create })} />
            {browsing && (
              <Button variant="ghost" className="mt-2" onClick={() => setBrowsing(false)}>Vazgeç</Button>
            )}
          </section>
        )}
      </div>
    </StepPage>
  )
}
