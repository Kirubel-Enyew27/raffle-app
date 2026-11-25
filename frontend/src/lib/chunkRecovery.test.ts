import test from 'node:test'
import assert from 'node:assert/strict'
import {
  attemptChunkRecovery,
  chunkUrlFromError,
  isChunkLoadError,
  loadRouteModule,
} from './chunkRecovery.ts'
import type { RecoveryStorage } from './chunkRecovery.ts'

const chunkError = (url = 'https://kwik.et/assets/AdminLivePage-F60NY5hl.js') =>
  new TypeError(`Failed to fetch dynamically imported module: ${url}`)

function memoryStorage(): RecoveryStorage & { values: Map<string, string> } {
  const values = new Map<string, string>()
  return {
    values,
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
  }
}

test('detects browser chunk-loading failures but not ordinary errors', () => {
  assert.equal(isChunkLoadError(chunkError()), true)
  assert.equal(isChunkLoadError(new Error('Importing a module script failed.')), true)
  assert.equal(isChunkLoadError({ name: 'ChunkLoadError', message: 'Loading chunk 12 failed' }), true)
  assert.equal(isChunkLoadError(new Error('Cannot read properties of undefined')), false)
  assert.equal(isChunkLoadError('network down'), false)
})

test('extracts the failing chunk URL when the browser includes it', () => {
  assert.equal(
    chunkUrlFromError(chunkError()),
    'https://kwik.et/assets/AdminLivePage-F60NY5hl.js',
  )
  assert.equal(
    chunkUrlFromError(new Error('Failed to fetch dynamically imported module: /assets/WalletPage-abc123.js')),
    '/assets/WalletPage-abc123.js',
  )
  assert.equal(chunkUrlFromError(new Error('something else')), null)
})

test('retries transient chunk failures and then succeeds', async () => {
  let attempts = 0
  const result = await loadRouteModule(
    async () => {
      attempts += 1
      if (attempts < 3) throw chunkError()
      return 'loaded'
    },
    { retryDelaysMs: [0, 0], sleep: async () => {} },
  )

  assert.equal(result, 'loaded')
  assert.equal(attempts, 3)
})

test('stops retrying after the bounded budget for a permanently missing chunk', async () => {
  let attempts = 0
  await assert.rejects(
    loadRouteModule(
      async () => {
        attempts += 1
        throw chunkError()
      },
      { retryDelaysMs: [0, 0], sleep: async () => {} },
    ),
    /Failed to fetch dynamically imported module/,
  )

  assert.equal(attempts, 3)
})

test('does not retry ordinary module evaluation errors', async () => {
  let attempts = 0
  await assert.rejects(
    loadRouteModule(
      async () => {
        attempts += 1
        throw new Error('bad module initialization')
      },
      { retryDelaysMs: [0, 0], sleep: async () => {} },
    ),
    /bad module initialization/,
  )

  assert.equal(attempts, 1)
})

test('refreshes at most once for a failed build and resets for a new build', () => {
  const storage = memoryStorage()
  let reloads = 0
  const reload = () => { reloads += 1 }

  assert.equal(
    attemptChunkRecovery(chunkError(), '/admin/live', { storage, reload, buildId: 'build-1' }),
    true,
  )
  assert.equal(
    attemptChunkRecovery(chunkError(), '/admin/users', { storage, reload, buildId: 'build-1' }),
    false,
  )
  assert.equal(reloads, 1)

  // A newer deployment gets a fresh guard and can recover once more.
  assert.equal(
    attemptChunkRecovery(chunkError(), '/admin/live', { storage, reload, buildId: 'build-2' }),
    true,
  )
  assert.equal(reloads, 2)
})

test('never auto-refreshes for ordinary application errors', () => {
  const storage = memoryStorage()
  let reloads = 0

  assert.equal(
    attemptChunkRecovery(new Error('render failed'), '/admin/live', {
      storage,
      reload: () => { reloads += 1 },
      buildId: 'build-1',
    }),
    false,
  )
  assert.equal(reloads, 0)
})

test('does not refresh without storage to enforce the loop guard', () => {
  let reloads = 0
  assert.equal(
    attemptChunkRecovery(chunkError(), '/admin/live', {
      storage: null,
      reload: () => { reloads += 1 },
      buildId: 'build-1',
    }),
    false,
  )
  assert.equal(reloads, 0)
})
