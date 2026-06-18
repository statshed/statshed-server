/**
 * AIDEV-NOTE: StatShed brand mascot — a dog whose headlamp is a live status LED.
 *
 * The dog is drawn with `currentColor` (set via a text color class) so it adapts to
 * light/dark mode automatically. The lamp LED sits in its own <g> that overrides
 * currentColor with the overall-health color (reusing HEALTH_STATUS_COLORS), and an
 * SVG blur filter turns that fill into a matching colored glow. While work is in
 * progress the lamp gently "breathes" (see .animate-mascot-pulse in index.css;
 * disabled under prefers-reduced-motion).
 *
 * Replaces the old "SD" monogram left over from the previous name, "StatDash".
 */
import { useId } from 'react'
import { cn } from '@/lib/utils'
import { HEALTH_STATUS_COLORS, HEALTH_STATUS_LABELS } from '@/lib/constants'
import type { HealthStatus } from '@/types'

// 'unknown' covers the window before /health has loaded (grey lamp — never green,
// which would falsely signal "all good" during an outage).
export type MascotStatus = HealthStatus | 'unknown'

const LED_COLOR_CLASS: Record<MascotStatus, string> = {
  ...HEALTH_STATUS_COLORS, // text-green-500 / text-red-500 / text-blue-500 / text-gray-400
  unknown: 'text-gray-400',
}

const LED_LABEL: Record<MascotStatus, string> = {
  ...HEALTH_STATUS_LABELS,
  unknown: 'Status unavailable',
}

interface MascotLogoProps {
  status: MascotStatus
  className?: string
}

export default function MascotLogo({ status, className }: MascotLogoProps) {
  // Unique filter id per instance so multiple mascots can't collide.
  const glowId = useId().replace(/:/g, '')
  const label = `StatShed — ${LED_LABEL[status]}`

  return (
    <svg
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      aria-label={label}
      className={cn('text-slate-700 dark:text-slate-200', className)}
    >
      <title>{label}</title>
      <defs>
        <filter id={glowId} x="-120%" y="-120%" width="340%" height="340%">
          <feGaussianBlur stdDeviation="1.6" result="b" />
          <feMerge>
            <feMergeNode in="b" />
            <feMergeNode in="b" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* floppy ears (behind the head) */}
      <path d="M17 14 C 7 13, 4 23, 7 32 C 8.5 36.5, 15.5 36, 18 31 Z" fill="currentColor" />
      <path d="M31 14 C 41 13, 44 23, 41 32 C 39.5 36.5, 32.5 36, 30 31 Z" fill="currentColor" />
      {/* inner-ear shading for depth */}
      <path d="M16.5 17 C 10.5 16.5, 9 24, 10.5 30 C 11.5 33, 15 32.6, 16.8 29 Z" fill="currentColor" opacity={0.38} />
      <path d="M31.5 17 C 37.5 16.5, 39 24, 37.5 30 C 36.5 33, 33 32.6, 31.2 29 Z" fill="currentColor" opacity={0.38} />

      {/* head */}
      <path
        d="M24 10.5 C 30 10.5, 33 14.5, 33 19.5 C 34.6 24, 33 30.5, 28.4 34 C 26.4 35.6, 21.6 35.6, 19.6 34 C 15 30.5, 13.4 24, 15 19.5 C 15 14.5, 18 10.5, 24 10.5 Z"
        fill="currentColor"
        opacity={0.9}
      />

      {/* muzzle */}
      <ellipse cx="24" cy="29.5" rx="7" ry="5.6" fill="currentColor" opacity={0.4} />

      {/* eyes + catchlights */}
      <circle cx="19.3" cy="23" r="2.05" fill="currentColor" />
      <circle cx="28.7" cy="23" r="2.05" fill="currentColor" />
      <circle cx="20.1" cy="22.3" r="0.7" fill="#fff" opacity={0.92} />
      <circle cx="29.5" cy="22.3" r="0.7" fill="#fff" opacity={0.92} />

      {/* nose + mouth */}
      <ellipse cx="24" cy="27" rx="2.5" ry="2" fill="currentColor" />
      <path
        d="M24 28.8 C 24 31, 22.4 31.8, 21 31.3 M24 28.8 C 24 31, 25.6 31.8, 27 31.3"
        stroke="currentColor"
        strokeWidth="1"
        fill="none"
        strokeLinecap="round"
      />

      {/* headlamp strap + lamp housing */}
      <path d="M13.5 18.2 C 17.5 13.5, 30.5 13.5, 34.5 18.2" stroke="currentColor" strokeWidth="2.4" fill="none" strokeLinecap="round" opacity={0.85} />
      <circle cx="24" cy="15" r="5.6" fill="currentColor" />
      <circle cx="24" cy="15" r="4.3" fill="currentColor" opacity={0.5} />

      {/* status LED — its own color; the blur filter turns the fill into a glow */}
      <g className={cn('transition-colors duration-500', LED_COLOR_CLASS[status])}>
        {status === 'in_progress' && (
          <circle className="animate-mascot-pulse" cx="24" cy="15" r="3.1" fill="currentColor" />
        )}
        <g filter={`url(#${glowId})`}>
          <circle cx="24" cy="15" r="3.1" fill="currentColor" />
        </g>
      </g>
      {/* lens highlight */}
      <circle cx="22.6" cy="13.7" r="0.95" fill="#fff" opacity={0.85} />
    </svg>
  )
}
