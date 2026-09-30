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
  return (
    <span className="flex min-w-0 items-start gap-1.5 text-sm">
      <CheckCircle size={16} weight="bold" className="mt-0.5 shrink-0 text-ok" aria-hidden />
      <span className="min-w-0">
        <span className="block">
          <span className="font-medium">Bağlandı</span> · <span className="font-mono uppercase">{check.protocol}</span> · yükleme <span className="font-mono">{upload}</span> · hash{' '}
          <span className="font-mono">{check.hash_method || 'yok'}</span>
        </span>
        {check.tools && check.tools.length > 0 && (
          <span className="block truncate font-mono text-xs text-muted-foreground" title={check.tools.join(' ')}>
            {check.tools.join(' ')}
          </span>
        )}
      </span>
    </span>
  )
}
