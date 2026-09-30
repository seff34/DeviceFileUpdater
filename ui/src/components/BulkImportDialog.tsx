import { FileArrowUp } from '@phosphor-icons/react'
import { useMemo, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { parseDeviceText } from '@/lib/csv'
import type { Device } from '@/lib/types'

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  existing: Device[]
  onApply: (next: Device[]) => void
}

export function BulkImportDialog({ open, onOpenChange, existing, onApply }: Props) {
  const [text, setText] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const parsed = useMemo(() => parseDeviceText(text), [text])
  const known = new Set(existing.map((d) => d.host.trim()))
  const fresh = parsed.devices.filter((d) => !known.has(d.host))
  const skipped = parsed.devices.length - fresh.length

  const close = (v: boolean) => {
    if (!v) setText('')
    onOpenChange(v)
  }
  const apply = (next: Device[]) => {
    onApply(next)
    close(false)
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Toplu cihaz ekle</DialogTitle>
          <DialogDescription>
            Excel'den satırları kopyalayıp yapıştırın ya da bir CSV dosyası seçin. Sütunlar sırasıyla: IP, kullanıcı adı, şifre. Başlık satırı ve <span className="font-mono">;</span> ile ayrılmış dosyalar da kabul edilir.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-2">
          <div className="flex items-center justify-between">
            <Label htmlFor="bulk-text">Cihaz satırları</Label>
            <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()}>
              <FileArrowUp aria-hidden /> CSV dosyası seç
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept=".csv,.txt,text/csv,text/plain"
              className="hidden"
              onChange={async (e) => {
                const f = e.target.files?.[0]
                if (f) setText(await f.text())
                e.target.value = ''
              }}
            />
          </div>
          <Textarea id="bulk-text" value={text} onChange={(e) => setText(e.target.value)} rows={10} spellCheck={false} className="font-mono text-sm" placeholder={'10.0.0.21\troot\tşifre\n10.0.0.22\troot\tşifre'} />
          {text.trim() && (
            <div className="text-sm" role="status">
              <p>
                {parsed.devices.length} cihaz okundu{skipped > 0 && `, ${skipped} tanesi listede zaten var ve atlanacak`}.
              </p>
              {parsed.errors.length > 0 && (
                <ul className="mt-1 max-h-24 overflow-y-auto text-fail">
                  {parsed.errors.map((e) => <li key={e}>{e}</li>)}
                </ul>
              )}
            </div>
          )}
        </div>
        <DialogFooter className="gap-2">
          <Button variant="outline" disabled={parsed.devices.length === 0} onClick={() => apply(parsed.devices)}>
            Listeyi bununla değiştir ({parsed.devices.length})
          </Button>
          <Button disabled={fresh.length === 0} onClick={() => apply([...existing, ...fresh])}>
            Listeye ekle ({fresh.length})
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
