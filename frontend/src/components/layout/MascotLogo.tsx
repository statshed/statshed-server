/**
 * AIDEV-NOTE: StatShed brand mascot — a dog whose headlamp is a live status LED.
 *
 * Design: the dog is a flat `currentColor` silhouette (it adapts to light/dark via the
 * text color set on the <svg>). The face (eyes, nose, mouth) and the separating outlines
 * of the headlamp strap + bezel are NEGATIVE SPACE — nested <g> wrappers override
 * `currentColor` with the HEADER BACKGROUND color (text-white dark:text-gray-800) so
 * those shapes punch through to the page behind. That negative space is what makes the
 * lamp read as a real device sitting on a strap, instead of (as the previous version did)
 * a glowing dot painted on the forehead: every shape was `currentColor` back then, so on a
 * same-color head only the LED was visible.
 *
 * AIDEV-NOTE: The knockout color is deliberately coupled to the header background
 * (bg-white dark:bg-gray-800 in Header.tsx). It also renders on NotFound.tsx, which
 * keeps the coupling valid by seating the mascot inside a `bg-white dark:bg-gray-800`
 * circle (same color). If it is ever placed on a DIFFERENT background, update the knockout
 * color here to match (or switch the knockouts to an SVG <mask> for true transparency) —
 * otherwise the face and lamp outlines will show the wrong color.
 *
 * The lamp lens uses the overall-health color (reusing HEALTH_STATUS_COLORS). While work
 * is in progress a crisp ring "pings" outward from the lamp (.animate-mascot-ping in
 * index.css; it rests as a static faint halo under prefers-reduced-motion).
 *
 * Replaces the old "SD" monogram left over from the previous name, "StatDash".
 */
import { cn } from '@/lib/utils'
import { HEALTH_STATUS_COLORS } from '@/lib/constants'
import type { HealthStatus } from '@/types'

// 'unknown' covers the window before /health has loaded (grey lamp — never green,
// which would falsely signal "all good" during an outage).
export type MascotStatus = HealthStatus | 'unknown'

const LED_COLOR_CLASS: Record<MascotStatus, string> = {
  ...HEALTH_STATUS_COLORS, // text-green-500 / text-red-500 / text-blue-500 / text-gray-400
  unknown: 'text-gray-400',
}

// Tailwind class that paints a shape in the header background color, turning it into
// negative space against the currentColor dog. See the AIDEV-NOTE above.
const KNOCKOUT = 'text-white dark:text-gray-800'

// Brow strap path — drawn twice (dog fill, then a knockout outline that separates it
// from the same-color head), so it lives in one place.
const STRAP_D = 'M12.6 18.3 Q 24 14.4 35.4 18.3 L 35.4 21 Q 24 17.1 12.6 21 Z'

export default function MascotLogo({
  status,
  className,
}: {
  status: MascotStatus
  className?: string
}) {
  // AIDEV-NOTE: Decorative (aria-hidden). The adjacent "StatShed" text labels the header
  // link, and overall status is conveyed by the lamp color plus the link's title tooltip
  // (set in Header), the connection pill, and the favicon. Keeping the SVG out of the a11y
  // tree also avoids a second "StatShed" text node that would break the e2e
  // getByText('StatShed') header assertion.
  return (
    <svg
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      focusable="false"
      className={cn('text-slate-700 dark:text-slate-200', className)}
    >
      {/* floppy ears (behind the head) */}
      <path d="M16 14 C 9 14, 5.5 22, 8 30 C 9.2 34, 15 33.4, 16.8 28 C 17.4 24, 17.2 18, 16 14 Z" fill="currentColor" />
      <path d="M32 14 C 39 14, 42.5 22, 40 30 C 38.8 34, 33 33.4, 31.2 28 C 30.6 24, 30.8 18, 32 14 Z" fill="currentColor" />

      {/* head */}
      <ellipse cx="24" cy="24.5" rx="11" ry="12.4" fill="currentColor" />

      {/* headlamp strap: dog fill, then a knockout outline to lift it off the head */}
      <path d={STRAP_D} fill="currentColor" />
      <g className={KNOCKOUT}>
        <path d={STRAP_D} fill="none" stroke="currentColor" strokeWidth="0.9" strokeLinejoin="round" />
      </g>

      {/* lamp bezel: dog fill (drawn after the strap so it occludes the strap outline
          beneath it), then its own knockout outline */}
      <circle cx="24" cy="16.4" r="5.2" fill="currentColor" />
      <g className={KNOCKOUT}>
        <circle cx="24" cy="16.4" r="5.2" fill="none" stroke="currentColor" strokeWidth="1" />
      </g>

      {/* face — eyes, nose, muzzle smile, all negative space */}
      <g className={KNOCKOUT}>
        <circle cx="19.4" cy="25.2" r="1.6" fill="currentColor" />
        <circle cx="28.6" cy="25.2" r="1.6" fill="currentColor" />
        <ellipse cx="24" cy="29.8" rx="2.3" ry="1.8" fill="currentColor" />
        <path
          d="M24 31.5 Q 24 33.5 22.2 33.3 M24 31.5 Q 24 33.5 25.8 33.3"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.1"
          strokeLinecap="round"
        />
      </g>

      {/* status LED — its own (health) color; smoothly transitions and perks up on hover */}
      <g className={cn('transition-colors duration-500', LED_COLOR_CLASS[status])}>
        {status === 'in_progress' && (
          <circle className="animate-mascot-ping" cx="24" cy="16.4" r="5.4" fill="none" stroke="currentColor" strokeWidth="1.4" />
        )}
        <circle cx="24" cy="16.4" r="3.1" fill="currentColor" />
      </g>

      {/* lens glint */}
      <circle cx="22.6" cy="15.3" r="0.85" fill="#fff" opacity={0.85} />
    </svg>
  )
}
