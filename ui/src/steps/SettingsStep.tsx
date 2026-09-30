import { CaretRight, WarningCircle } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { SaveIndicator } from '@/components/SaveIndicator'
import { StepPage } from '@/components/StepPage'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'
import { navigate } from '@/lib/router'
import type { Settings } from '@/lib/types'
import { saveBlocker, useDraft } from '@/lib/useDraft'
import { cn } from '@/lib/utils'
import { validateSettings } from '@/lib/validate'

const POLICY: Record<Settings['post_command_policy'], string> = {
  on_change: 'Sadece dosya değiştiyse',
  always: 'Her zaman',
  never: 'Hiçbir zaman',
}

function Field({ id, label, help, error, children }: { id: string; label: string; help: ReactNode; error?: string; children: ReactNode }) {
  return (
    <div className="grid gap-x-8 gap-y-1.5 py-5 md:grid-cols-[minmax(0,18rem)_minmax(0,1fr)]">
      <div>
        <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
        <p id={`${id}-help`} className="mt-1 text-sm text-muted-foreground">{help}</p>
      </div>
      <div className="max-w-md">
        {children}
        {error && <p id={`${id}-err`} className="mt-1 text-xs text-fail">{error}</p>}
      </div>
    </div>
  )
}

const POLICY_HELP: Record<Settings['post_command_policy'], string> = {
  on_change: 'Sadece dosya değiştiyse: hiçbir dosya güncellenmediyse komut çalışmaz.',
  always: 'Her zaman: dosyalar aynı olsa da çalışır.',
  never: 'Hiçbir zaman: komut çalıştırılmaz.',
}

const toInt = (v: string) => (v.trim() === '' ? Number.NaN : Number(v))
const shown = (n: number) => (Number.isNaN(n) ? '' : String(n))

export function SettingsStep() {
  const q = useQuery({ queryKey: ['settings'], queryFn: api.settings })
  const { draft, setDraft, state, error } = useDraft<Settings>({
    queryKey: ['settings'],
    data: q.data,
    save: api.saveSettings,
    valid: (s) => Object.keys(validateSettings(s)).length === 0,
  })
  const [advanced, setAdvanced] = useState(false)
  const ready = q.isSuccess && !!draft
  const errs = draft ? validateSettings(draft) : {}
  const set = (patch: Partial<Settings>) => draft && setDraft({ ...draft, ...patch })
  const blocker = q.isPending
    ? 'Ayarlar yükleniyor.'
    : !ready
      ? 'Ayarlar yüklenemedi.'
      : Object.keys(errs).length > 0
        ? 'Ayarlardaki hataları düzeltin.'
        : saveBlocker(state, error)
  const advErrors = (errs.connect_timeout_sec ? 1 : 0) + (errs.command_timeout_sec ? 1 : 0)
  const described = (id: string, err?: string) => (err ? `${id}-help ${id}-err` : `${id}-help`)

  return (
    <StepPage
      step="settings"
      action={{ onBack: () => navigate('/files'), onPrimary: () => navigate('/preview'), blocker }}
      aside={ready ? <SaveIndicator state={state} error={error} /> : undefined}
    >
      {q.isPending ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-16" />)}</div>
      ) : !ready || !draft ? (
        <Alert variant="destructive"><AlertDescription>{(q.error as Error | null)?.message ?? 'Ayarlar yüklenemedi.'}</AlertDescription></Alert>
      ) : (
        <div className="divide-y rounded-md border bg-card px-6">
          <Field id="set-parallel" label="Paralel cihaz sayısı" help="Aynı anda güncellenen cihaz sayısı. Ağ yavaşsa düşürün." error={errs.parallel}>
            <Input
              id="set-parallel"
              type="number"
              min={1}
              max={200}
              value={shown(draft.parallel)}
              onChange={(e) => set({ parallel: toInt(e.target.value) })}
              aria-invalid={!!errs.parallel}
              aria-describedby={described('set-parallel', errs.parallel)}
              className="w-28 tabular-nums"
            />
          </Field>
          <Field
            id="set-backup"
            label="Yedek al"
            help={<>Değişecek dosyanın eski hali cihazda <span className="font-mono">&lt;hedef&gt;.bak</span> olarak saklanır. Her çalıştırmada üzerine yazılır.</>}
          >
            <Switch id="set-backup" checked={draft.backup} onCheckedChange={(v) => set({ backup: v })} aria-describedby="set-backup-help" />
          </Field>
          <Field id="set-post" label="post-command" help={
              <>
                Dosyalar işlendikten sonra her cihazda, giriş yapılan kullanıcının yetkileriyle çalışacak komut. Boş bırakılırsa çalışmaz. Çıktısı ve çıkış kodu rapora yazılır.
                <span className="mt-1 block text-xs">Not: komut her cihazda çalışır, önce tek bir cihazda denemeniz önerilir.</span>
              </>
            }>
            <Input
              id="set-post"
              value={draft.post_command}
              onChange={(e) => set({ post_command: e.target.value })}
              placeholder="systemctl restart app"
              className="font-mono"
              spellCheck={false}
              aria-describedby="set-post-help"
            />
            <div className="mt-3">
              <Label htmlFor="set-policy" className="mb-1.5 block text-xs text-muted-foreground">Ne zaman çalışsın</Label>
              <Select value={draft.post_command_policy} onValueChange={(v) => set({ post_command_policy: v as Settings['post_command_policy'] })} disabled={!draft.post_command.trim()}>
                <SelectTrigger id="set-policy" className="w-64" aria-describedby="set-policy-help"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {Object.entries(POLICY).map(([k, v]) => <SelectItem key={k} value={k}>{v}</SelectItem>)}
                </SelectContent>
              </Select>
              <p id="set-policy-help" className="mt-1.5 text-xs text-muted-foreground">
                {draft.post_command.trim() ? POLICY_HELP[draft.post_command_policy] : 'Önce komut girin.'}
              </p>
            </div>
          </Field>

          <Collapsible open={advanced} onOpenChange={setAdvanced}>
            <CollapsibleTrigger className="flex w-full items-center gap-1.5 rounded-md py-4 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring">
              <CaretRight size={14} className={cn('transition-transform duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none', advanced && 'rotate-90')} aria-hidden />
              Gelişmiş
              {advErrors > 0 && (
                <span className="ml-2 flex items-center gap-1 text-xs font-normal text-warn">
                  <WarningCircle size={14} aria-hidden />
                  ({advErrors} hata)
                </span>
              )}
            </CollapsibleTrigger>
            <CollapsibleContent className="divide-y border-t">
              <Field id="set-connect" label="Bağlantı zaman aşımı (sn)" help="Cihaza bağlanmak için beklenecek en uzun süre." error={errs.connect_timeout_sec}>
                <Input
                  id="set-connect"
                  type="number"
                  min={1}
                  value={shown(draft.connect_timeout_sec)}
                  onChange={(e) => set({ connect_timeout_sec: toInt(e.target.value) })}
                  aria-invalid={!!errs.connect_timeout_sec}
                  aria-describedby={described('set-connect', errs.connect_timeout_sec)}
                  className="w-28 tabular-nums"
                />
              </Field>
              <Field id="set-command" label="Komut zaman aşımı (sn)" help="Cihazda çalışan tek bir komut veya aktarım için en uzun süre. Büyük dosyalarda artırın." error={errs.command_timeout_sec}>
                <Input
                  id="set-command"
                  type="number"
                  min={1}
                  value={shown(draft.command_timeout_sec)}
                  onChange={(e) => set({ command_timeout_sec: toInt(e.target.value) })}
                  aria-invalid={!!errs.command_timeout_sec}
                  aria-describedby={described('set-command', errs.command_timeout_sec)}
                  className="w-28 tabular-nums"
                />
              </Field>
              <Field
                id="set-strict"
                label="Host key doğrulaması"
                help={<>Açıkken her cihazın SSH anahtarı ilk bağlantıda çalışma alanındaki <span className="font-mono">known_hosts</span> dosyasına kaydedilir. Anahtar sonradan değişirse bağlantı reddedilir. Kapalıyken cihaz anahtarı doğrulanmaz.</>}
              >
                <Switch id="set-strict" checked={draft.strict_host_key} onCheckedChange={(v) => set({ strict_host_key: v })} aria-describedby="set-strict-help" />
              </Field>
            </CollapsibleContent>
          </Collapsible>
        </div>
      )}
    </StepPage>
  )
}
