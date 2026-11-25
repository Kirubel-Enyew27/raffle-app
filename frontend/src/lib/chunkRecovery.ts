/**
 * Recovery helpers for Vite-generated dynamic imports.
 *
 * A chunk request can fail because the network dropped, because a CDN edge is
 * still propagating, or because the HTML the user is running references a
 * build whose hashed assets no longer exist. The first two are worth retrying;
 * the last is only fixed by loading the current entry point. Keep the retry
 * count and automatic refreshes bounded so a permanently missing chunk cannot
 * trap the browser in a reload loop.
 */

const CHUNK_ERROR_PATTERNS = [
  /failed to fetch dynamically imported module/i,
  /error loading dynamically imported module/i,
  /importing a module script failed/i,
  /unable to preload (?:css|module)/i,
  /chunkloaderror/i,
  /loading (?:css )?chunk [\w-]+ failed/i,
]

const DEFAULT_RETRY_DELAYS_MS = [250, 750] as const
const RECOVERY_STORAGE_PREFIX = 'kwik:chunk-recovery:'

export type RecoveryStorage = Pick<Storage, 'getItem' | 'setItem'>

export interface ChunkRetryOptions {
  /** Delays between attempts. The length is the maximum number of retries. */
  retryDelaysMs?: readonly number[]
  sleep?: (ms: number) => Promise<void>
}

export interface ChunkRecoveryOptions {
  storage?: RecoveryStorage | null
  reload?: () => void
  buildId?: string
}

function errorMessage(error: unknown): string {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string') return message
  }
  return ''
}

/** True only for the browser errors produced by a failed script/chunk load. */
export function isChunkLoadError(error: unknown): boolean {
  if (error && typeof error === 'object' && 'name' in error) {
    if ((error as { name?: unknown }).name === 'ChunkLoadError') return true
  }
  const message = errorMessage(error)
  return CHUNK_ERROR_PATTERNS.some(pattern => pattern.test(message))
}

/** The failing asset URL when the browser included one in the error message. */
export function chunkUrlFromError(error: unknown): string | null {
  const message = errorMessage(error)
  const absolute = message.match(/https?:\/\/[^\s)"']+?\.m?js(?:\?[^\s)"']*)?/i)
  if (absolute) return absolute[0]
  const relative = message.match(/(?:^|[\s"'(])(\/[^\s)"']+?\.m?js(?:\?[^\s)"']*)?)/i)
  return relative?.[1] ?? null
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

/**
 * Load a route module, retrying only chunk-load failures. Ordinary module
 * evaluation errors are surfaced immediately so real application bugs are not
 * disguised as network problems.
 */
export async function loadRouteModule<T>(
  loader: () => Promise<T>,
  options: ChunkRetryOptions = {},
): Promise<T> {
  const delays = options.retryDelaysMs ?? DEFAULT_RETRY_DELAYS_MS
  const sleep = options.sleep ?? defaultSleep

  for (let attempt = 0; ; attempt += 1) {
    try {
      return await loader()
    } catch (error) {
      if (!isChunkLoadError(error) || attempt >= delays.length) throw error
      await sleep(delays[attempt])
    }
  }
}

function getSessionStorage(): RecoveryStorage | null {
  try {
    return typeof window !== 'undefined' ? window.sessionStorage : null
  } catch {
    return null
  }
}

function currentBuildId(): string {
  return typeof __KWIK_BUILD_ID__ === 'string' && __KWIK_BUILD_ID__
    ? __KWIK_BUILD_ID__
    : 'development'
}

function reloadPage(): void {
  if (typeof window === 'undefined') return
  window.location.reload()
}

/**
 * Reload the current entry point at most once for a given build in this tab.
 *
 * The guard is deliberately build-scoped rather than route-scoped: a broken
 * build should cause one reload attempt, not one per page the user opens. A
 * new deployment gets a new build id and therefore a fresh recovery attempt.
 * Authentication storage is untouched, so the reload keeps the user signed in.
 */
export function attemptChunkRecovery(
  error: unknown,
  routeKey: string,
  options: ChunkRecoveryOptions = {},
): boolean {
  if (!isChunkLoadError(error)) return false

  const storage = options.storage === undefined ? getSessionStorage() : options.storage
  if (!storage) return false

  const buildId = options.buildId ?? currentBuildId()
  const key = `${RECOVERY_STORAGE_PREFIX}${buildId}`

  try {
    if (storage.getItem(key)) return false
    storage.setItem(key, routeKey)
  } catch {
    return false
  }

  const reload = options.reload ?? reloadPage
  try {
    reload()
    return true
  } catch {
    return false
  }
}
