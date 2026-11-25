import test from 'node:test'
import assert from 'node:assert/strict'
import { lazyRoute } from './lazyRoute.ts'
import type { RecoveryStorage } from '../lib/chunkRecovery.ts'

const chunkError = () =>
  new TypeError('Failed to fetch dynamically imported module: https://kwik.et/assets/AdminLivePage-F60NY5hl.js')

function memoryStorage(): RecoveryStorage {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
  }
}

test('lazy routes recover transient chunk failures without refreshing', async () => {
  let attempts = 0
  let reloads = 0
  const load = lazyRoute(
    '/admin/live',
    async () => {
      attempts += 1
      if (attempts < 2) throw chunkError()
      return { Component: () => null }
    },
    {
      retry: { retryDelaysMs: [0], sleep: async () => {} },
      recovery: {
        storage: memoryStorage(),
        buildId: 'build-1',
        reload: () => { reloads += 1 },
      },
    },
  )

  const module = await load()
  assert.equal(attempts, 2)
  assert.equal(reloads, 0)
  assert.equal(typeof module.Component, 'function')
})

test('lazy routes refresh once and surface the error when a chunk stays missing', async () => {
  let attempts = 0
  let reloads = 0
  const load = lazyRoute(
    '/admin/live',
    async () => {
      attempts += 1
      throw chunkError()
    },
    {
      retry: { retryDelaysMs: [0, 0], sleep: async () => {} },
      recovery: {
        storage: memoryStorage(),
        buildId: 'build-1',
        reload: () => { reloads += 1 },
      },
    },
  )

  await assert.rejects(load(), /Failed to fetch dynamically imported module/)
  assert.equal(attempts, 3)
  assert.equal(reloads, 1)
})

test('lazy routes do not refresh for ordinary module errors', async () => {
  let reloads = 0
  const load = lazyRoute(
    '/admin/live',
    async () => {
      throw new Error('module evaluation failed')
    },
    {
      retry: { retryDelaysMs: [0, 0], sleep: async () => {} },
      recovery: {
        storage: memoryStorage(),
        buildId: 'build-1',
        reload: () => { reloads += 1 },
      },
    },
  )

  await assert.rejects(load(), /module evaluation failed/)
  assert.equal(reloads, 0)
})
