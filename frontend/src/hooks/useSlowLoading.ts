/**
 * AIDEV-NOTE: Returns true once `isLoading` has stayed true for `delayMs`, so a hung/slow
 * backend can surface a "taking longer than usual" hint instead of only skeletons. Resets
 * to false as soon as loading ends (or restarts the timer if loading toggles).
 */

import { useEffect, useState } from 'react'

export function useSlowLoading(isLoading: boolean, delayMs = 5000): boolean {
  const [isSlow, setIsSlow] = useState(false)

  useEffect(() => {
    if (!isLoading) {
      setIsSlow(false)
      return
    }
    const id = setTimeout(() => setIsSlow(true), delayMs)
    return () => clearTimeout(id)
  }, [isLoading, delayMs])

  return isSlow
}
