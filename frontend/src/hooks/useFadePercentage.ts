/**
 * AIDEV-NOTE: Hook to calculate job card fade opacity based on expiration time
 *
 * Fade calculation:
 * - No fade (opacity 1.0) until 50% of job lifetime has passed
 * - Gradually fades from 100% to 50% opacity during the second half of lifetime
 * - The fade_start is the midpoint between updated_at and expires_at
 */

import { useState, useEffect } from 'react'

interface FadeResult {
  /** Current opacity value from 1.0 (fully visible) to 0.5 (faded) */
  opacity: number
  /** Whether the job is currently in the fade window (< 50% time remaining) */
  isFading: boolean
  /** Time remaining until expiration in milliseconds */
  timeRemainingMs: number
  /** Human-readable time remaining string (e.g., "2h 30m") */
  timeRemainingText: string
  /** Whether the expiration date is valid (use to guard UI rendering) */
  isValid: boolean
}

/**
 * Calculate fade opacity and time remaining for a job approaching expiration
 *
 * @param expiresAt - ISO date string when the job expires (null if no expiration)
 * @param updatedAt - ISO date string when the job was last updated
 * @param updateIntervalMs - How often to recalculate (default 30 seconds)
 */
export function useFadePercentage(
  expiresAt: string | null,
  updatedAt: string,
  updateIntervalMs: number = 30000
): FadeResult {
  const [result, setResult] = useState<FadeResult>(() =>
    calculateFade(expiresAt, updatedAt)
  )

  useEffect(() => {
    // Recalculate immediately when inputs change
    const initialResult = calculateFade(expiresAt, updatedAt)
    setResult(initialResult)

    // AIDEV-NOTE: Skip interval if no expiration, invalid dates, or already expired
    // This prevents unnecessary timers for jobs that won't change
    if (!expiresAt || !initialResult.isValid || initialResult.timeRemainingMs <= 0) {
      return
    }

    // Update on interval for live countdown
    const interval = setInterval(() => {
      setResult(calculateFade(expiresAt, updatedAt))
    }, updateIntervalMs)

    return () => clearInterval(interval)
  }, [expiresAt, updatedAt, updateIntervalMs])

  return result
}

/**
 * Pure function to calculate fade values
 */
function calculateFade(expiresAt: string | null, updatedAt: string): FadeResult {
  // No expiration = no fade
  if (!expiresAt) {
    return {
      opacity: 1.0,
      isFading: false,
      timeRemainingMs: Infinity,
      timeRemainingText: 'never',
      isValid: true,
    }
  }

  const now = Date.now()
  const expiresAtMs = new Date(expiresAt).getTime()
  const updatedAtMs = new Date(updatedAt).getTime()

  // Guard against invalid dates
  if (isNaN(expiresAtMs) || isNaN(updatedAtMs)) {
    return {
      opacity: 1.0,
      isFading: false,
      timeRemainingMs: Infinity,
      timeRemainingText: 'unknown',
      isValid: false,
    }
  }

  const timeRemainingMs = Math.max(0, expiresAtMs - now)

  // Calculate fade_start as midpoint between updated_at and expires_at
  // AIDEV-NOTE: This means fading begins when 50% of the job's lifetime has elapsed
  const fadeStartMs = (updatedAtMs + expiresAtMs) / 2

  let opacity: number
  let isFading: boolean

  if (now < fadeStartMs) {
    // Before fade window - full opacity
    opacity = 1.0
    isFading = false
  } else if (now >= expiresAtMs) {
    // At or past expiration - minimum opacity
    opacity = 0.5
    isFading = true
  } else {
    // In fade window - interpolate from 1.0 to 0.5
    const fadeWindowMs = expiresAtMs - fadeStartMs
    const progressInWindow = (now - fadeStartMs) / fadeWindowMs
    // Clamp progress to [0, 1] for safety
    const clampedProgress = Math.min(1, Math.max(0, progressInWindow))
    opacity = 1.0 - clampedProgress * 0.5 // 1.0 → 0.5
    isFading = true
  }

  return {
    opacity,
    isFading,
    timeRemainingMs,
    timeRemainingText: formatTimeRemaining(timeRemainingMs),
    isValid: true,
  }
}

/**
 * Format milliseconds to human-readable time string
 */
function formatTimeRemaining(ms: number): string {
  if (ms === Infinity) return 'never'
  if (ms <= 0) return 'expired'

  const totalSeconds = Math.floor(ms / 1000)
  const totalMinutes = Math.floor(totalSeconds / 60)
  const totalHours = Math.floor(totalMinutes / 60)
  const totalDays = Math.floor(totalHours / 24)

  if (totalDays > 0) {
    const hours = totalHours % 24
    return hours > 0 ? `${totalDays}d ${hours}h` : `${totalDays}d`
  }
  if (totalHours > 0) {
    const minutes = totalMinutes % 60
    return minutes > 0 ? `${totalHours}h ${minutes}m` : `${totalHours}h`
  }
  if (totalMinutes > 0) {
    return `${totalMinutes}m`
  }
  return `${totalSeconds}s`
}
