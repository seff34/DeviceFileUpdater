import { PlugsConnected } from '@phosphor-icons/react'
import { memo } from 'react'
import { StatusBadge } from '@/components/StatusBadge'
import { baseName } from '@/lib/format'
import { filterDevices, type MatrixFilter } from '@/lib/summary'
import type { DeviceResult, RunResult } from '@/lib/types'

const Row = memo(function Row({ d, files }: { d: DeviceResult; files: string[] }) {
  return (
    <tr className="hover:bg-muted/50">
      <th scope="row" className="sticky left-0 border-b bg-card px-3 py-2 text-left font-mono font-normal whitespace-nowrap">
        {d.host}
      </th>
      {d.error ? (
        <td colSpan={files.length} className="border-b px-3 py-2 text-fail">
          <span className="flex items-start gap-1.5">
            <PlugsConnected size={16} weight="bold" className="mt-0.5 shrink-0" aria-hidden />
            <span>{d.error}</span>
          </span>
        </td>
      ) : (
        files.map((remote) => {
          const fr = d.files.find((x) => x.remote === remote)
          return (
            <td key={remote} className="border-b px-3 py-2 align-top">
              {fr ? (
                <>
                  <StatusBadge status={fr.status} />
                  {fr.error && <p className="mt-1 max-w-56 text-xs text-fail break-words">{fr.error}</p>}
                </>
              ) : (
                <span className="text-muted-foreground">-</span>
              )}
            </td>
          )
        })
      )}
    </tr>
  )
})

/** Device × file grid. Rows are devices; a device that could not connect gets one full-width reason cell. */
export function PreviewMatrix({ result, filter }: { result: RunResult; filter: MatrixFilter }) {
  const rows = filterDevices(result, filter)
  if (rows.length === 0) return <p className="rounded-md border bg-card p-6 text-sm text-muted-foreground">Bu filtreye uyan cihaz yok.</p>
  return (
    <div className="max-h-[60vh] overflow-auto rounded-md border bg-card" tabIndex={0} role="region" aria-label="Cihaz ve dosya tablosu">
      <table className="w-full border-separate border-spacing-0 text-sm">
        <thead className="sticky top-0 z-10 bg-card">
          <tr>
            <th scope="col" className="sticky left-0 z-20 border-b bg-card px-3 py-2 text-left font-medium">Cihaz</th>
            {result.files.map((f) => (
              <th key={f} scope="col" className="border-b px-3 py-2 text-left font-medium" title={f}>
                <span className="block max-w-48 truncate font-mono text-xs">{baseName(f)}</span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((d) => (
            <Row key={d.host} d={d} files={result.files} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

export type { MatrixFilter }
