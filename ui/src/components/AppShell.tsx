import { ClockCounterClockwise, FolderSimple } from '@phosphor-icons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { api } from '@/lib/api'
import { Link } from '@/lib/router'

export function AppShell({ children, stepper }: { children: ReactNode; stepper?: ReactNode }) {
  const ws = useQuery({ queryKey: ['workspace'], queryFn: api.workspace })
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="h-14 border-b bg-card">
        <div className="mx-auto flex h-full max-w-6xl items-center gap-4 px-6">
          <Link to="/" className="font-semibold tracking-tight outline-none focus-visible:ring-2 focus-visible:ring-ring">
            DeviceFileUpdater
          </Link>
          {ws.data?.current && (
            <span className="flex min-w-0 items-center gap-1.5 text-sm text-muted-foreground" title={ws.data.current}>
              <FolderSimple size={16} aria-hidden />
              <span className="truncate font-mono">{ws.data.current}</span>
            </span>
          )}
          <Link
            to="/history"
            className="ml-auto flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 text-sm text-muted-foreground hover:text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ClockCounterClockwise size={16} aria-hidden /> Geçmiş raporlar
          </Link>
        </div>
      </header>
      {stepper}
      {children}
    </div>
  )
}
