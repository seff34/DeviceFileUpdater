import { CheckCircle, CircleNotch, XCircle } from '@phosphor-icons/react'
import { ErrorText } from '@/components/ErrorText'
import type { CheckResult } from '@/lib/types'

export type CheckView = CheckResult | 'testing' | undefined

export function ConnectionCell({ check }: { check: CheckView }) {
  if (check === undefined) return <span className="text-sm text-muted-foreground">Test edilmedi</span>
  if (check === 'testing')
    return (
      <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <CircleNotch size={14} className="animate-spin" aria-hidden /> Test ediliyor
      </span>
    )
  if (!check.ok)
    return (
      <span className="flex min-w-0 items-start gap-1.5 text-sm text-fail">
        <XCircle size={16} weight="bold" className="mt-0.5 shrink-0" aria-hidden />
        <ErrorText raw={check.error ?? ''} />
      </span>
    )
  const upload = check.upload_methods?.[0] ?? 'yok'
  const tools = check.tools?.length ? `\nAraçlar: ${check.tools.join(' ')}` : ''
  // Two short lines that fit the row's input height, so a long fleet stays scannable; the
  // full detail (tool list) is in the tooltip.
  return (
    <span className="flex min-w-0 items-start gap-1.5 text-sm" title={`Yükleme: ${upload} · Hash: ${check.hash_method || 'yok'}${tools}`}>
      <CheckCircle size={16} weight="bold" className="mt-0.5 shrink-0 text-ok" aria-hidden />
      <span className="min-w-0">
        <span className="block"><span className="font-medium">Bağlandı</span> · <span className="font-mono uppercase">{check.protocol}</span></span>
        <span className="block truncate font-mono text-xs text-muted-foreground">
          yükleme {upload} · hash {check.hash_method || 'yok'}
        </span>
      </span>
    </span>
  )
}
