import { DownloadSimple, Eye, EyeSlash, Plus, PlugsConnected, Rows, Trash, WarningCircle } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { BulkImportDialog } from '@/components/BulkImportDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { ConnectionCell, type CheckView } from '@/components/ConnectionCell'
import { SaveIndicator } from '@/components/SaveIndicator'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { api, EXPORT_DEVICES_URL, testConnection } from '@/lib/api'
import { navigate } from '@/lib/router'
import type { CheckResult, Device } from '@/lib/types'
import { saveBlocker, useDraft } from '@/lib/useDraft'
import { validateDevices } from '@/lib/validate'

const allValid = (ds: Device[]) => validateDevices(ds).every((r) => r === null)

export function DevicesStep() {
  const q = useQuery({ queryKey: ['devices'], queryFn: api.devices })
  const { draft = [], setDraft, state, error } = useDraft<Device[]>({ queryKey: ['devices'], data: q.data, save: api.saveDevices, valid: allValid })
  const errors = useMemo(() => validateDevices(draft), [draft])
  const [touched, setTouched] = useState<Set<string>>(new Set())
  const [reveal, setReveal] = useState<Set<number>>(new Set())
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [checks, setChecks] = useState<Record<string, CheckView>>({})
  const [testing, setTesting] = useState(false)
  const [bulkOpen, setBulkOpen] = useState(false)
  const [confirmRemove, setConfirmRemove] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  useEffect(() => () => abortRef.current?.abort(), [])

  const hosts = draft.map((d) => d.host.trim())
  const unreachable = hosts.filter((h) => {
    const c = checks[h]
    return c !== undefined && c !== 'testing' && !c.ok
  })
  const selectedHosts = hosts.filter((h) => selected.has(h))
  const ready = q.isSuccess
  const canTest = ready && state === 'saved' && draft.length > 0 && errors.every((e) => e === null) && !testing

  const update = (i: number, patch: Partial<Device>) => {
    const old = draft[i].host.trim()
    // A changed credential or host invalidates that row's earlier test result.
    setChecks((c) => {
      if (!(old in c)) return c
      const rest = { ...c }
      delete rest[old]
      return rest
    })
    setDraft(draft.map((d, j) => (j === i ? { ...d, ...patch } : d)))
  }
  const apply = (next: Device[]) => {
    setReveal(new Set())
    setTouched(new Set())
    setDraft(next)
  }
  const remove = (i: number) => {
    setDraft(draft.filter((_, j) => j !== i))
    setReveal(new Set())
    setTouched(new Set())
  }
  const add = () => {
    const n = draft.length
    setDraft([...draft, { host: '', username: draft.at(-1)?.username ?? '', password: '' }])
    requestAnimationFrame(() => document.getElementById(`dev-host-${n}`)?.focus())
  }
  const touch = (key: string) => setTouched((t) => (t.has(key) ? t : new Set(t).add(key)))
  const fieldError = (i: number, f: keyof Device) => {
    const e = errors[i]?.[f]
    if (!e) return undefined
    // "required" errors wait for the operator to leave the field; conflicts show at once.
    const value = draft[i][f].trim()
    return value || touched.has(`${i}:${f}`) ? e : undefined
  }

  const runTest = async (only: string[]) => {
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    const targets = only.length ? only : hosts
    setChecks((c) => ({ ...c, ...Object.fromEntries(targets.map((h) => [h, 'testing' as const])) }))
    setTesting(true)
    try {
      await testConnection(only, (r: CheckResult) => setChecks((c) => ({ ...c, [r.host]: r })), ac.signal)
    } catch (e) {
      if (!ac.signal.aborted) toast.error((e as Error).message)
    } finally {
      setChecks((c) => Object.fromEntries(Object.entries(c).filter(([, v]) => v !== 'testing')))
      if (abortRef.current === ac) setTesting(false)
    }
  }

  const blocker =
    draft.length === 0
      ? 'En az bir cihaz ekleyin.'
      : errors.some((e) => e !== null)
        ? 'Cihaz listesindeki hataları düzeltin.'
        : saveBlocker(state, error)

  const allSelected = draft.length > 0 && selectedHosts.length === draft.length

  return (
    <StepPage
      step="devices"
      action={{ onBack: () => navigate('/workspace'), onPrimary: () => navigate('/files'), blocker }}
      aside={<div className="flex items-center gap-4"><span className="text-sm tabular-nums text-muted-foreground">{draft.length} cihaz</span>{ready && <SaveIndicator state={state} error={error} />}</div>}
    >
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Button variant="outline" onClick={add} disabled={!ready}><Plus aria-hidden /> Cihaz ekle</Button>
        <Button variant="outline" onClick={() => setBulkOpen(true)} disabled={!ready}><Rows aria-hidden /> Toplu ekle</Button>
        {ready ? (
          <Button variant="outline" asChild>
            <a href={EXPORT_DEVICES_URL} download="devices.csv"><DownloadSimple aria-hidden /> CSV dışa aktar</a>
          </Button>
        ) : (
          <Button variant="outline" disabled><DownloadSimple aria-hidden /> CSV dışa aktar</Button>
        )}
        <div className="ml-auto flex items-center gap-2">
          {selectedHosts.length > 0 && (
            <Button variant="outline" disabled={!canTest} onClick={() => runTest(selectedHosts)}>
              <PlugsConnected aria-hidden /> Seçilileri test et ({selectedHosts.length})
            </Button>
          )}
          <Button variant="outline" disabled={!canTest} onClick={() => runTest([])} aria-busy={testing}>
            <PlugsConnected aria-hidden /> Tümünü test et
          </Button>
        </div>
      </div>

      {unreachable.length > 0 && !testing && (
        <Alert className="mb-3 border-warn/30 bg-warn/5">
          <WarningCircle className="text-warn" aria-hidden />
          <AlertDescription className="flex flex-wrap items-center justify-between gap-3 text-foreground">
            <span>{unreachable.length} cihaza bağlanılamadı. Listeden çıkarabilir veya olduğu gibi devam edebilirsiniz; bu cihazlar raporda başarısız görünür.</span>
            <Button variant="outline" size="sm" onClick={() => setConfirmRemove(true)}>Ulaşılamayanları çıkar</Button>
          </AlertDescription>
        </Alert>
      )}

      {q.isPending ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-10" />)}</div>
      ) : q.isError ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error).message}</AlertDescription></Alert>
      ) : draft.length === 0 ? (
        <div className="rounded-md border border-dashed bg-card p-10 text-center">
          <p className="font-medium">Henüz cihaz yok</p>
          <p className="mt-1 text-sm text-muted-foreground">Cihazları tek tek ekleyin veya Excel'den IP, kullanıcı adı ve şifre sütunlarını yapıştırın.</p>
          <div className="mt-4 flex justify-center gap-2">
            <Button onClick={add}><Plus aria-hidden /> Cihaz ekle</Button>
            <Button variant="outline" onClick={() => setBulkOpen(true)}><Rows aria-hidden /> Toplu ekle</Button>
          </div>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border bg-card">
          <Table className="min-w-[60rem] table-fixed">
            <TableHeader>
              <TableRow>
                <TableHead className="w-10">
                  <Checkbox
                    aria-label="Tümünü seç"
                    checked={allSelected ? true : selectedHosts.length > 0 ? 'indeterminate' : false}
                    onCheckedChange={(v) => setSelected(v === true ? new Set(hosts) : new Set())}
                  />
                </TableHead>
                <TableHead className="w-10 text-right">#</TableHead>
                <TableHead className="w-48">IP</TableHead>
                <TableHead className="w-40">Kullanıcı adı</TableHead>
                <TableHead className="w-56">Şifre</TableHead>
                <TableHead>Bağlantı</TableHead>
                <TableHead className="w-12"><span className="sr-only">İşlemler</span></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {draft.map((d, i) => {
                const n = i + 1
                const hostErr = fieldError(i, 'host')
                const userErr = fieldError(i, 'username')
                const h = d.host.trim()
                return (
                  <TableRow key={i} className="align-top">
                    <TableCell className="pt-3">
                      <Checkbox
                        aria-label={`Satır ${n} seç`}
                        checked={selected.has(h)}
                        disabled={!h}
                        onCheckedChange={(v) => setSelected((s) => { const x = new Set(s); if (v === true) x.add(h); else x.delete(h); return x })}
                      />
                    </TableCell>
                    <TableCell className="pt-3.5 text-right font-mono text-xs text-muted-foreground">{n}</TableCell>
                    <TableCell>
                      <Input
                        id={`dev-host-${i}`}
                        aria-label={`IP, satır ${n}`}
                        value={d.host}
                        onChange={(e) => update(i, { host: e.target.value })}
                        onBlur={() => touch(`${i}:host`)}
                        aria-invalid={!!hostErr}
                        aria-describedby={hostErr ? `dev-host-err-${i}` : undefined}
                        className="h-9 font-mono"
                        spellCheck={false}
                        placeholder="örn. 10.0.0.21"
                      />
                      {hostErr && <p id={`dev-host-err-${i}`} className="mt-1 text-xs text-fail">{hostErr}</p>}
                    </TableCell>
                    <TableCell>
                      <Input
                        aria-label={`Kullanıcı adı, satır ${n}`}
                        value={d.username}
                        onChange={(e) => update(i, { username: e.target.value })}
                        onBlur={() => touch(`${i}:username`)}
                        aria-invalid={!!userErr}
                        aria-describedby={userErr ? `dev-user-err-${i}` : undefined}
                        className="h-9"
                        spellCheck={false}
                        autoComplete="off"
                      />
                      {userErr && <p id={`dev-user-err-${i}`} className="mt-1 text-xs text-fail">{userErr}</p>}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Input
                          aria-label={`Şifre, satır ${n}`}
                          type={reveal.has(i) ? 'text' : 'password'}
                          value={d.password}
                          onChange={(e) => update(i, { password: e.target.value })}
                          className="h-9 font-mono"
                          autoComplete="off"
                          spellCheck={false}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={reveal.has(i) ? 'Şifreyi gizle' : 'Şifreyi göster'}
                          onClick={() => setReveal((s) => { const x = new Set(s); if (x.has(i)) x.delete(i); else x.add(i); return x })}
                        >
                          {reveal.has(i) ? <EyeSlash aria-hidden /> : <Eye aria-hidden />}
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell className="whitespace-normal pt-2.5"><ConnectionCell check={checks[h]} /></TableCell>
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

      <BulkImportDialog open={bulkOpen} onOpenChange={setBulkOpen} existing={draft} onApply={apply} />
      <ConfirmDialog
        open={confirmRemove}
        onOpenChange={setConfirmRemove}
        title="Ulaşılamayan cihazlar çıkarılsın mı?"
        description={<>Bağlantı testinde başarısız olan {unreachable.length} cihaz listeden silinecek: <span className="font-mono">{unreachable.join(', ')}</span></>}
        confirmLabel="Çıkar"
        destructive
        onConfirm={() => apply(draft.filter((d) => !unreachable.includes(d.host.trim())))}
      />
    </StepPage>
  )
}
