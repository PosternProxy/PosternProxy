import axios from 'axios'

/**
 * Best-effort extraction of a user-facing error message from any thrown value.
 *
 * Order of preference:
 *   1. Axios error → response.data.error (the JSON shape our backend returns)
 *   2. Axios error → response.statusText / message
 *   3. Any Error → .message
 *   4. fallback
 *
 * Use this in catch blocks instead of `catch (e: any)` to satisfy
 * @typescript-eslint/no-explicit-any while preserving the friendly message.
 */
export function errorMessage(e: unknown, fallback = 'Something went wrong'): string {
  if (axios.isAxiosError(e)) {
    const data = e.response?.data as { error?: string } | undefined
    return data?.error ?? e.response?.statusText ?? e.message ?? fallback
  }
  if (e instanceof Error) return e.message
  return fallback
}
