import { attemptChunkRecovery, loadRouteModule } from '../lib/chunkRecovery.ts'
import type { ChunkRecoveryOptions, ChunkRetryOptions } from '../lib/chunkRecovery.ts'

export interface LazyRouteOptions {
  retry?: ChunkRetryOptions
  recovery?: ChunkRecoveryOptions
}

/**
 * React Router lazy route loader with bounded retries and a single
 * build-scoped refresh for stale chunk failures.
 */
export function lazyRoute<T>(
  routeKey: string,
  loader: () => Promise<T>,
  options: LazyRouteOptions = {},
): () => Promise<T> {
  return async () => {
    try {
      return await loadRouteModule(loader, options.retry)
    } catch (error) {
      attemptChunkRecovery(error, routeKey, options.recovery)
      throw error
    }
  }
}
