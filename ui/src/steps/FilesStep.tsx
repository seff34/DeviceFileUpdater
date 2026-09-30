import { FileArrowUp, Trash, UploadSimple } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useRef, useState, type DragEvent } from 'react'
import { toast } from 'sonner'
import { SaveIndicator } from '@/components/SaveIndicator'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { api } from '@/lib/api'
import { baseName, formatBytes } from '@/lib/format'
import { navigate } from '@/lib/router'
import type { ManifestRow } from '@/lib/types'
import { saveBlocker, useDraft } from '@/lib/useDraft'
import { cn } from '@/lib/utils'
import { validateManifest } from '@/lib/validate'

const strip = (rows: ManifestRow[]) => rows.map(({ local_path, remote_path, mode }) => ({ local_path, remote_path, mode }))
const rowsValid = (rows: ManifestRow[]) => validateManifest(rows).every((e) => e === null) && rows.every((r) => r.file.exists)

export function FilesStep() {
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['manifest'], queryFn: api.manifest })
  const { draft = [], setDraft, state, error } = useDraft<ManifestRow[]>({
    queryKey: ['manifest'],
    data: q.data,
    save: (rows) => api.saveManifest(strip(rows)),
    valid: rowsValid,
  })
  const errors = useMemo(() => validateManifest(draft), [draft])
  const [touched, setTouched] = useState<Set<string>>(new Set())
  const [dragging, setDragging] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const ready = q.isSuccess

  const upload = useMutation({
    mutationFn: (files: File[]) => api.uploadFiles(files),
    onSuccess: (up) => {
      const merged = [...draft]
      let firstNew = -1
      for (const u of up) {
        const file = { exists: true, size: u.size, sha256: u.sha256 }
        const i = merged.findIndex((r) => r.local_path === u.local_path)
        if (i >= 0) merged[i] = { ...merged[i], file }
        else {
          if (firstNew < 0) firstNew = merged.length
          merged.push({ local_path: u.local_path, remote_path: '', mode: '', file })
        }
      }
      setDraft(merged)
      void qc.invalidateQueries({ queryKey: ['run'] })
      const replaced = up.filter((u) => u.replaced).length
      toast.success(replaced > 0 ? `${up.length} dosya yüklendi, ${replaced} tanesi eskisinin yerine geçti.` : `${up.length} dosya yüklendi.`)
      if (firstNew >= 0) requestAnimationFrame(() => document.getElementById(`mf-remote-${firstNew}`)?.focus())
    },
    onError: (e) => toast.error((e as Error).message),
  })

  const canUpload = ready && !upload.isPending
  const pick = (list: FileList | null) => {
    const files = list ? Array.from(list) : []
    if (files.length && canUpload) upload.mutate(files)
  }
  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragging(false)
    pick(e.dataTransfer.files)
  }

  const update = (i: number, patch: Partial<ManifestRow>) => setDraft(draft.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  const remove = (i: number) => {
    setDraft(draft.filter((_, j) => j !== i))
    setTouched(new Set())
  }
  const touch = (key: string) => setTouched((t) => (t.has(key) ? t : new Set(t).add(key)))
  const fieldError = (i: number, f: 'remote_path' | 'mode') => {
    const e = errors[i]?.[f]
    if (!e) return undefined
    return draft[i][f].trim() || touched.has(`${i}:${f}`) ? e : undefined
  }
  // A target ending in "/" is a folder: append the uploaded file's name.
  const completeRemote = (i: number) => {
    touch(`${i}:remote_path`)
    const r = draft[i]
    if (r.remote_path.trim().endsWith('/')) update(i, { remote_path: r.remote_path.trim() + baseName(r.local_path) })
  }

  const totalSize = draft.reduce((n, r) => n + r.file.size, 0)
  const blocker =
    draft.length === 0
      ? 'En az bir dosya ekleyin.'
      : !rowsValid(draft)
        ? 'Dosya satırlarındaki hataları düzeltin.'
        : saveBlocker(state, error)

  return (
    <StepPage
      step="files"
      action={{ onBack: () => navigate('/devices'), onPrimary: () => navigate('/settings'), blocker }}
      aside={
        <div className="flex items-center gap-4">
          <span className="text-sm tabular-nums text-muted-foreground">{draft.length} dosya · {formatBytes(totalSize)}</span>
          {ready && <SaveIndicator state={state} error={error} />}
        </div>
      }
    >
      <div
        data-dropzone
        onDragOver={(e) => {
          e.preventDefault()
          if (canUpload) setDragging(true)
        }}
        onDragLeave={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false)
        }}
        onDrop={onDrop}
        className="relative mb-4 flex flex-wrap items-center gap-4 rounded-md border border-dashed bg-card px-5 py-4"
      >
        {/* Highlight is an overlay so only opacity animates, never border or background. */}
        <div
          aria-hidden
          className={cn(
            'pointer-events-none absolute inset-0 rounded-md border-2 border-primary bg-primary/5 opacity-0 transition-opacity duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none',
            dragging && 'opacity-100',
          )}
        />
        <UploadSimple
          size={24}
          className={cn('text-muted-foreground transition-colors duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none', dragging && 'text-primary')}
          aria-hidden
        />
        <div className="min-w-0 flex-1 text-sm">
          <p className="font-medium">{upload.isPending ? 'Yükleniyor' : dragging ? 'Yüklemek için bırakın' : 'Dosyaları buraya bırakın'}</p>
          <p className="text-muted-foreground">
            Dosyalar çalışma alanındaki <span className="font-mono">files/</span> klasörüne kopyalanır. Aynı adlı dosya yenisiyle değiştirilir.
          </p>
        </div>
        <Button variant="outline" disabled={!canUpload} onClick={() => inputRef.current?.click()} aria-busy={upload.isPending}>
          <FileArrowUp aria-hidden /> Dosya seç
        </Button>
        <input
          ref={inputRef}
          type="file"
          multiple
          aria-label="Dosya yükle"
          className="hidden"
          disabled={!canUpload}
          onChange={(e) => {
            pick(e.target.files)
            e.target.value = ''
          }}
        />
      </div>

      <p className="mb-2 text-xs text-muted-foreground">
        Cihazdaki dosya aynıysa dokunulmaz, farklıysa güncellenir, yoksa oluşturulur. Sonu <span className="font-mono">/</span> ile biten hedefe dosya adı eklenir. İzin boş bırakılırsa mevcut dosyanın izni korunur, yeni dosya <span className="font-mono">0644</span> olur.
      </p>

      {q.isPending ? (
        <div className="space-y-2">{[0, 1].map((i) => <Skeleton key={i} className="h-10" />)}</div>
      ) : q.isError ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error).message}</AlertDescription></Alert>
      ) : draft.length === 0 ? (
        <div className="rounded-md border bg-card p-10 text-center">
          <p className="font-medium">Henüz dosya yok</p>
          <p className="mt-1 text-sm text-muted-foreground">Cihazlara gönderilecek dosyaları yukarıdaki alana bırakın veya seçin.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-10 text-right">#</TableHead>
                <TableHead className="min-w-56">Dosya</TableHead>
                <TableHead className="min-w-72">Cihazdaki hedef yol</TableHead>
                <TableHead className="w-28">İzin</TableHead>
                <TableHead className="w-12"><span className="sr-only">İşlemler</span></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {draft.map((r, i) => {
                const n = i + 1
                const remoteErr = fieldError(i, 'remote_path')
                const modeErr = fieldError(i, 'mode')
                return (
                  <TableRow key={r.local_path + i} className="align-top">
                    <TableCell className="pt-3.5 text-right font-mono text-xs text-muted-foreground">{n}</TableCell>
                    <TableCell className="pt-2.5">
                      <p className="truncate font-mono text-sm" title={r.local_path}>{baseName(r.local_path)}</p>
                      {r.file.exists ? (
                        <p className="font-mono text-xs text-muted-foreground">
                          {formatBytes(r.file.size)} · sha256 {r.file.sha256.slice(0, 8)}
                        </p>
                      ) : (
                        <p className="text-xs text-fail">Dosya bulunamadı: {r.local_path}. Yeniden yükleyin veya satırı silin.</p>
                      )}
                    </TableCell>
                    <TableCell>
                      <Input
                        id={`mf-remote-${i}`}
                        aria-label={`Hedef yol, satır ${n}`}
                        value={r.remote_path}
                        onChange={(e) => update(i, { remote_path: e.target.value })}
                        onBlur={() => completeRemote(i)}
                        aria-invalid={!!remoteErr}
                        aria-describedby={remoteErr ? `mf-remote-err-${i}` : undefined}
                        placeholder={`/opt/app/${baseName(r.local_path)}`}
                        className="h-9 font-mono"
                        spellCheck={false}
                      />
                      {remoteErr && <p id={`mf-remote-err-${i}`} className="mt-1 text-xs text-fail">{remoteErr}</p>}
                    </TableCell>
                    <TableCell>
                      <Input
                        aria-label={`İzin, satır ${n}`}
                        value={r.mode}
                        onChange={(e) => update(i, { mode: e.target.value })}
                        onBlur={() => touch(`${i}:mode`)}
                        aria-invalid={!!modeErr}
                        aria-describedby={modeErr ? `mf-mode-err-${i}` : undefined}
                        placeholder="koru"
                        inputMode="numeric"
                        className="h-9 font-mono"
                      />
                      {modeErr && <p id={`mf-mode-err-${i}`} className="mt-1 text-xs text-fail">{modeErr}</p>}
                    </TableCell>
                    <TableCell>
                      <Button type="button" variant="ghost" size="icon" aria-label={`Satır ${n} sil`} onClick={() => remove(i)}>
                        <Trash aria-hidden />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </StepPage>
  )
}
