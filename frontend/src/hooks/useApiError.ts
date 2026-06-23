// Extracts a display-ready message from a React Query error.
export function useApiError(error: unknown): string | null {
  if (!error) return null
  if (error instanceof Error) return error.message
  return 'An unexpected error occurred'
}
