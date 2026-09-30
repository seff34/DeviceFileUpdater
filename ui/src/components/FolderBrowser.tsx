import { ArrowUp, Folder, FolderPlus, HardDrives, SealCheck } from '@phosphor-icons/react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'

function join(dir: string, name: string) {
  const sep = dir.includes('\\') ? '\\' : '/'
  return dir.endsWith(sep) ? dir + name : dir + sep + name
}

export function FolderBrowser({ onOpen, busy }: { onOpen: (path: string, create: boolean) => void; busy: boolean }) {
  const [path, setPath] = useState<string | undefined>(undefined)
  const [typed, setTyped] = useState('')
  const [newName, setNewName] = useState('')
  const q = useQuery({ queryKey: ['fs', path ?? ''], queryFn: () => api.fs(path), placeholderData: keepPreviousData })
  const cur = q.data

  // Sync the address bar to the listing during render (adjust-state-on-change pattern).
  const [syncedPath, setSyncedPath] = useState<string | undefined>(undefined)
  if (cur && cur.path !== syncedPath) {
    setSyncedPath(cur.path)
    setTyped(cur.path)
  }

  // One filled button at a time: creating wins once a name is typed; using the listed folder
  // is primary only when it already is a workspace, so a first-timer is never nudged into
  // turning their home folder into one.
  const creating = !!newName.trim()
  const useIsPrimary = !creating && !!cur?.is_workspace

  const nameError = /[\\/]/.test(newName) ? 'Klasör adı / veya \\ içeremez.' : newName.trim() === '.' || newName.trim() === '..' ? 'Geçersiz klasör adı.' : null

  const go = (e: FormEvent) => {
    e.preventDefault()
    if (typed.trim()) setPath(typed.trim())
  }
  const create = (e: FormEvent) => {
    e.preventDefault()
    if (cur && newName.trim() && !nameError) onOpen(join(cur.path, newName.trim()), true)
  }

  return (
    <div className="rounded-md border bg-card">
      <form onSubmit={go} className="flex items-center gap-2 border-b p-3">
        <Button type="button" variant="outline" size="icon" aria-label="Üst klasör" disabled={!cur?.parent || cur.parent === cur.path} onClick={() => setPath(cur!.parent)}>
          <ArrowUp aria-hidden />
        </Button>
        <Label htmlFor="fb-path" className="sr-only">Klasör yolu</Label>
        <Input id="fb-path" value={typed} onChange={(e) => setTyped(e.target.value)} className="font-mono" spellCheck={false} />
        <Button type="submit" variant="outline">Git</Button>
      </form>

      {cur && cur.roots.length > 0 && (
        <div className="flex flex-wrap gap-1 border-b px-3 py-2">
          {cur.roots.map((r) => (
            <Button key={r} type="button" variant="ghost" size="sm" className="font-mono" onClick={() => setPath(r)}>
              <HardDrives aria-hidden /> {r}
            </Button>
          ))}
        </div>
      )}

      <div className="max-h-80 overflow-y-auto" aria-busy={q.isFetching}>
        {q.isError ? (
          <Alert variant="destructive" className="m-3 w-auto">
            <AlertDescription className="flex items-center justify-between gap-3">
              {(q.error as Error).message}
              <Button variant="outline" size="sm" onClick={() => setPath(undefined)}>Ana klasöre dön</Button>
            </AlertDescription>
          </Alert>
        ) : q.isPending ? (
          <div className="space-y-2 p-3">{[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8" />)}</div>
        ) : cur!.entries.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">Bu klasörde alt klasör yok.</p>
        ) : (
          <ul className="divide-y">
            {cur!.entries.map((e) => (
              <li key={e.path}>
                <button
                  type="button"
                  onClick={() => setPath(e.path)}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-muted focus-visible:bg-muted outline-none"
                >
                  <Folder size={16} className="shrink-0 text-muted-foreground" aria-hidden />
                  <span className="truncate font-mono">{e.name}</span>
                  {e.is_workspace && (
                    <span className="ml-auto inline-flex shrink-0 items-center gap-1 rounded-md border border-ok/25 bg-ok/10 px-1.5 py-0.5 text-xs text-ok">
                      <SealCheck size={12} weight="bold" aria-hidden /> Çalışma alanı
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-wrap items-end gap-3 border-t p-3">
        <form onSubmit={create} className="flex min-w-0 flex-1 items-end gap-2">
          <div className="min-w-0 flex-1">
            <Label htmlFor="fb-new" className="mb-1 block text-xs text-muted-foreground">Yeni klasör adı</Label>
            <Input id="fb-new" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="örn. hat-3-guncelleme" aria-invalid={!!nameError} aria-describedby={nameError ? 'fb-new-err' : undefined} />
            {nameError && <p id="fb-new-err" className="mt-1 text-xs text-fail">{nameError}</p>}
          </div>
          <Button type="submit" variant={creating ? 'default' : 'outline'} disabled={!cur || !newName.trim() || !!nameError || busy}>
            <FolderPlus aria-hidden /> Oluştur ve kullan
          </Button>
        </form>
        <Button type="button" variant={useIsPrimary ? 'default' : 'outline'} disabled={!cur || busy} onClick={() => onOpen(cur!.path, true)}>
          Bu klasörü kullan
        </Button>
      </div>
    </div>
  )
}
