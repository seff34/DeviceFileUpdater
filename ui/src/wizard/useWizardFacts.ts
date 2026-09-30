import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { validateDevices, validateManifest } from '@/lib/validate'
import type { WizardFacts } from '@/lib/steps'

/** Server-backed facts for step gating. Queries are shared with the steps (same keys). */
export function useWizardFacts(previewConfirmed: boolean): { facts: WizardFacts; loading: boolean } {
  const ws = useQuery({ queryKey: ['workspace'], queryFn: api.workspace })
  const hasWs = !!ws.data?.current
  const devices = useQuery({ queryKey: ['devices'], queryFn: api.devices, enabled: hasWs })
  const manifest = useQuery({ queryKey: ['manifest'], queryFn: api.manifest, enabled: hasWs })
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings, enabled: hasWs })
  const run = useQuery({ queryKey: ['run'], queryFn: api.runStatus, enabled: hasWs })
  const ds = devices.data ?? []
  const es = manifest.data ?? []
  return {
    loading: ws.isPending,
    facts: {
      workspace: hasWs,
      devices: ds.length,
      devicesValid: validateDevices(ds).every((r) => r === null),
      files: es.length,
      filesValid: validateManifest(es).every((r) => r === null) && es.every((e) => e.file.exists),
      settingsValid: settings.isSuccess,
      run: run.data ?? null,
      previewConfirmed,
    },
  }
}
