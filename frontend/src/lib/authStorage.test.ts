import test from 'node:test'
import assert from 'node:assert/strict'

import { createAuthStorage, pickStore, TOKEN_KEY, USER_KEY } from './authStorage.ts'

// Minimal in-memory Storage stand-in (node --test has no DOM).
function fakeStore(): Storage {
  const map = new Map<string, string>()
  return {
    get length() {
      return map.size
    },
    clear: () => map.clear(),
    getItem: (k: string) => map.get(k) ?? null,
    key: (i: number) => Array.from(map.keys())[i] ?? null,
    removeItem: (k: string) => void map.delete(k),
    setItem: (k: string, v: string) => void map.set(k, v),
  }
}

const local = fakeStore()
const session = fakeStore()

test('dev keeps the session per tab, production per browser', () => {
  assert.equal(pickStore(true, local, session), session)
  assert.equal(pickStore(false, local, session), local)
})

test('saving writes token and serialised user', () => {
  const s = fakeStore()
  const auth = createAuthStorage(s)
  auth.save('jwt-123', { id: 'u1', full_name: 'Player Two' })

  assert.equal(s.getItem(TOKEN_KEY), 'jwt-123')
  assert.deepEqual(JSON.parse(s.getItem(USER_KEY)!), { id: 'u1', full_name: 'Player Two' })
  assert.equal(auth.getToken(), 'jwt-123')
  assert.deepEqual(JSON.parse(auth.getUser()!), { id: 'u1', full_name: 'Player Two' })
})

test('two tabs (separate session stores) hold independent accounts', () => {
  const tabA = createAuthStorage(fakeStore())
  const tabB = createAuthStorage(fakeStore())

  tabA.save('token-a', { id: 'a', full_name: 'Player Two' })
  tabB.save('token-b', { id: 'b', full_name: 'Player Three' })

  assert.equal(tabA.getToken(), 'token-a')
  assert.equal(tabB.getToken(), 'token-b')

  tabB.clear()
  assert.equal(tabB.getToken(), null)
  assert.equal(tabA.getToken(), 'token-a', 'logging out one tab must not log out the other')
})

test('clear removes both keys and leaves unrelated entries alone', () => {
  const s = fakeStore()
  const auth = createAuthStorage(s)
  auth.save('jwt', { id: 'u1' })
  s.setItem('theme', 'dark')

  auth.clear()

  assert.equal(auth.getToken(), null)
  assert.equal(auth.getUser(), null)
  assert.equal(s.getItem('theme'), 'dark')
})
