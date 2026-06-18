/**
 * AIDEV-NOTE: Dynamic favicon hook
 * Updates the browser favicon based on health status
 * - Green: healthy/ok state
 * - Red: error/unhealthy state
 * - Grey: unknown state (backend unreachable / health not yet loaded) — must NOT show green
 *   during an outage, which would falsely signal "all good".
 */

import { useEffect } from 'react'

export type FaviconStatus = 'healthy' | 'error' | 'unknown'

/**
 * Creates an SVG favicon as a data URL
 */
function createFaviconSvg(color: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
    <circle cx="16" cy="16" r="14" fill="${color}" />
    <circle cx="16" cy="16" r="10" fill="${color}" stroke="white" stroke-width="2" opacity="0.8" />
  </svg>`
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

const FAVICON_COLORS: Record<FaviconStatus, string> = {
  healthy: '#22c55e', // green-500
  error: '#ef4444',   // red-500
  unknown: '#9ca3af', // gray-400
}

/**
 * Hook to dynamically update the favicon based on status
 * @param status - 'healthy' (green), 'error' (red), or 'unknown' (grey, backend unreachable)
 */
export function useFavicon(status: FaviconStatus): void {
  useEffect(() => {
    // Find or create the favicon link element
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')

    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      link.type = 'image/svg+xml'
      document.head.appendChild(link)
    }

    // Update the favicon
    link.href = createFaviconSvg(FAVICON_COLORS[status])
  }, [status])
}
