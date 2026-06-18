/**
 * AIDEV-NOTE: Base HTTP client for API calls
 * - Uses fetch with timeout support
 * - Handles JSON parsing and error responses
 * - Uses relative URLs with Vite proxy for development
 */

const DEFAULT_TIMEOUT = 30000 // 30 seconds

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public data?: unknown
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  timeout?: number
  body?: unknown
}

/**
 * Base fetch wrapper with timeout and error handling
 */
export async function apiRequest<T>(
  endpoint: string,
  options: RequestOptions = {}
): Promise<T> {
  const { timeout = DEFAULT_TIMEOUT, body, ...fetchOptions } = options

  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...fetchOptions.headers,
  }

  try {
    const response = await fetch(`/api${endpoint}`, {
      ...fetchOptions,
      headers,
      body: body ? JSON.stringify(body) : undefined,
      signal: controller.signal,
    })

    clearTimeout(timeoutId)

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      // AIDEV-NOTE: Prefer the backend's human-readable `message`, then the (often
      // machine-code) `error`, then a status fallback — so toasts/error screens show
      // actionable text. Guard against non-string values so a JSON object/number can't
      // become the message.
      const message =
        (typeof errorData.message === 'string' && errorData.message) ||
        (typeof errorData.error === 'string' && errorData.error) ||
        `Request failed with status ${response.status}`
      throw new ApiError(message, response.status, errorData)
    }

    // Handle 204 No Content
    if (response.status === 204) {
      return undefined as T
    }

    // AIDEV-NOTE: Parse the success body in its own try so a malformed 2xx response
    // (an HTML error page, truncated JSON, etc.) surfaces as a distinct server error
    // carrying the real HTTP status — instead of being misclassified as the status-0
    // "network error" the outer catch assigns to fetch-level failures.
    // Only a SyntaxError means "2xx but the body isn't valid JSON" (a server/contract
    // problem). A body stream that drops mid-flight throws a TypeError — a genuine
    // transport failure — so rethrow it to the outer catch, which classifies it as a
    // status-0 network error rather than mislabeling it a malformed response.
    try {
      return (await response.json()) as T
    } catch (parseError) {
      if (parseError instanceof SyntaxError) {
        throw new ApiError('Received a malformed response from the server', response.status)
      }
      throw parseError
    }
  } catch (error) {
    clearTimeout(timeoutId)

    if (error instanceof ApiError) {
      throw error
    }

    if (error instanceof Error) {
      if (error.name === 'AbortError') {
        throw new ApiError('Request timed out', 408)
      }
      throw new ApiError(error.message, 0)
    }

    throw new ApiError('An unknown error occurred', 0)
  }
}

// Convenience methods
export const api = {
  get: <T>(endpoint: string, options?: RequestOptions) =>
    apiRequest<T>(endpoint, { ...options, method: 'GET' }),

  post: <T>(endpoint: string, body?: unknown, options?: RequestOptions) =>
    apiRequest<T>(endpoint, { ...options, method: 'POST', body }),

  put: <T>(endpoint: string, body?: unknown, options?: RequestOptions) =>
    apiRequest<T>(endpoint, { ...options, method: 'PUT', body }),

  delete: <T>(endpoint: string, options?: RequestOptions) =>
    apiRequest<T>(endpoint, { ...options, method: 'DELETE' }),
}
