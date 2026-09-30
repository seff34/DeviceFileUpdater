import { createElement, useSyncExternalStore, type AnchorHTMLAttributes, type MouseEvent } from 'react'

const listeners = new Set<() => void>()

function subscribe(l: () => void) {
  listeners.add(l)
  window.addEventListener('popstate', l)
  return () => {
    listeners.delete(l)
    window.removeEventListener('popstate', l)
  }
}

export function navigate(to: string, opts: { replace?: boolean } = {}) {
  if (window.location.pathname === to) return
  if (opts.replace) window.history.replaceState(null, '', to)
  else window.history.pushState(null, '', to)
  listeners.forEach((l) => l())
  window.scrollTo?.(0, 0)
}

export function usePath() {
  return useSyncExternalStore(subscribe, () => window.location.pathname)
}

export function reportIdFromPath(path: string): string | null {
  const m = /^\/history\/([0-9]{8}-[0-9]{6}(?:-[0-9a-f]{4})?)$/.exec(path)
  return m ? m[1] : null
}

type LinkProps = AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }

export function Link({ to, onClick, ...rest }: LinkProps) {
  return createElement('a', {
    ...rest,
    href: to,
    onClick: (e: MouseEvent<HTMLAnchorElement>) => {
      onClick?.(e)
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
      e.preventDefault()
      navigate(to)
    },
  })
}
