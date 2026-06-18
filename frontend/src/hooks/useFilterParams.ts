/**
 * AIDEV-NOTE: URL params filter hook
 * Syncs filter state with URL search params for persistence and shareability
 */

import { useCallback, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'

interface UseFilterParamsOptions<T extends string> {
  /** URL parameter name for the filter */
  paramName: string
  /** Default value when param is not present */
  defaultValue: T
  /** Valid values for the filter */
  validValues: readonly T[]
}

/**
 * Hook to sync a filter value with URL search params
 * Provides persistence across page reloads and enables shareable URLs
 */
export function useFilterParam<T extends string>({
  paramName,
  defaultValue,
  validValues,
}: UseFilterParamsOptions<T>): [T, (value: T) => void] {
  const [searchParams, setSearchParams] = useSearchParams()

  // Get current value from URL, falling back to default
  const rawValue = searchParams.get(paramName)
  const isValidValue = rawValue && validValues.includes(rawValue as T)
  const currentValue = isValidValue ? (rawValue as T) : defaultValue

  // AIDEV-NOTE: Clean up invalid URL params to avoid confusing shareable links
  // If a param exists but is invalid, remove it from the URL
  useEffect(() => {
    if (rawValue && !isValidValue) {
      setSearchParams((prev) => {
        const newParams = new URLSearchParams(prev)
        newParams.delete(paramName)
        return newParams
      }, { replace: true })
    }
  }, [rawValue, isValidValue, paramName, setSearchParams])

  // Set the filter value in URL params
  const setValue = useCallback((value: T) => {
    setSearchParams((prev) => {
      const newParams = new URLSearchParams(prev)
      if (value === defaultValue) {
        // Remove param if it's the default value to keep URLs clean
        newParams.delete(paramName)
      } else {
        newParams.set(paramName, value)
      }
      return newParams
    }, { replace: true })
  }, [paramName, defaultValue, setSearchParams])

  return [currentValue, setValue]
}

/**
 * Hook to sync a search query with URL search params
 */
export function useSearchParam(paramName: string = 'q'): [string, (value: string) => void] {
  const [searchParams, setSearchParams] = useSearchParams()

  const currentValue = searchParams.get(paramName) ?? ''

  const setValue = useCallback((value: string) => {
    setSearchParams((prev) => {
      const newParams = new URLSearchParams(prev)
      if (value === '') {
        newParams.delete(paramName)
      } else {
        newParams.set(paramName, value)
      }
      return newParams
    }, { replace: true })
  }, [paramName, setSearchParams])

  return [currentValue, setValue]
}
