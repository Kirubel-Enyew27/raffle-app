import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createEndSession } from './endSession.ts'

function fakeStorage(refreshToken: string | null) {
  let current = refreshToken
  const cleared: string[] = []
  return {
    storage: {
      getRefreshToken: () => current,
      clear: () => {
        current = null
        cleared.push('cleared')
      },
    },
    /** What the storage holds right now (null once cleared). */
    token: () => current,
    clearedCount: () => cleared.length,
  }
}

test('logout revokes the refresh token and drops local credentials', async () => {
  const fake = fakeStorage('refresh-abc')
  const revoked: string[] = []

  const endSession = createEndSession({
    storage: fake.storage,
    revoke: async (token) => {
      revoked.push(token)
    },
  })

  await endSession()

  assert.deepEqual(revoked, ['refresh-abc'], 'the stored refresh token is revoked server-side')
  assert.equal(fake.token(), null, 'local credentials are gone')
})

test('local credentials are cleared before the revocation request settles', async () => {
  const fake = fakeStorage('refresh-abc')
  let resolveRequest: () => void = () => {}
  const pending = new Promise<void>((resolve) => {
    resolveRequest = resolve
  })

  const endSession = createEndSession({
    storage: fake.storage,
    revoke: () => pending,
  })

  const done = endSession()
  // The user is signed out immediately, not after the network round trip.
  assert.equal(fake.token(), null)
  assert.equal(fake.clearedCount(), 1)

  resolveRequest()
  await done
})

test('a failed revocation still signs the user out locally', async () => {
  const fake = fakeStorage('refresh-abc')
  let calls = 0

  const endSession = createEndSession({
    storage: fake.storage,
    revoke: async () => {
      calls += 1
      throw new Error('network down')
    },
  })

  await assert.doesNotReject(() => endSession())
  assert.equal(calls, 1)
  assert.equal(fake.token(), null)
})

test('logging out without a refresh token does not call the server', async () => {
  const fake = fakeStorage(null)
  let calls = 0

  const endSession = createEndSession({
    storage: fake.storage,
    revoke: async () => {
      calls += 1
    },
  })

  await endSession()
  assert.equal(calls, 0)
  assert.equal(fake.token(), null)
})
