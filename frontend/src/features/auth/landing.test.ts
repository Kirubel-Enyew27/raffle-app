import test from 'node:test'
import assert from 'node:assert/strict'
import {
  ADMIN_DASHBOARD_PATH,
  GAMES_PATH,
  landingPathForRole,
  resolvePostAuthPath,
  sanitizeRedirect,
  readPendingRedirect,
} from './landing.ts'

test('players land on games and admins on the admin dashboard', () => {
  assert.equal(landingPathForRole('user'), GAMES_PATH)
  assert.equal(landingPathForRole('admin'), ADMIN_DASHBOARD_PATH)
})

test('an unknown or missing role is treated as a player, never as an admin', () => {
  assert.equal(landingPathForRole(undefined), GAMES_PATH)
  assert.equal(landingPathForRole(null), GAMES_PATH)
  assert.equal(landingPathForRole(''), GAMES_PATH)
  assert.equal(landingPathForRole('superadmin'), GAMES_PATH)
  assert.equal(landingPathForRole('ADMIN'), GAMES_PATH)
})

test('resolvePostAuthPath falls back to the role landing page', () => {
  assert.equal(resolvePostAuthPath({ role: 'user' }), GAMES_PATH)
  assert.equal(resolvePostAuthPath({ role: 'admin' }), ADMIN_DASHBOARD_PATH)
  assert.equal(resolvePostAuthPath({}), GAMES_PATH)
})

test('resolvePostAuthPath honours an internal redirect for either role', () => {
  assert.equal(resolvePostAuthPath({ redirect: '/games/match/42', role: 'user' }), '/games/match/42')
  assert.equal(resolvePostAuthPath({ redirect: '/wallet', role: 'admin' }), '/wallet')
})

test('resolvePostAuthPath ignores unsafe redirects instead of following them', () => {
  assert.equal(resolvePostAuthPath({ redirect: '//evil.example.com', role: 'user' }), GAMES_PATH)
  assert.equal(resolvePostAuthPath({ redirect: 'https://evil.example.com', role: 'user' }), GAMES_PATH)
  assert.equal(resolvePostAuthPath({ redirect: '/\\evil.example.com', role: 'admin' }), ADMIN_DASHBOARD_PATH)
  assert.equal(resolvePostAuthPath({ redirect: '', role: 'user' }), GAMES_PATH)
})

test('sanitizeRedirect is re-exported for the Google helpers', () => {
  assert.equal(sanitizeRedirect('/games?join=ABC123'), '/games?join=ABC123')
  assert.equal(sanitizeRedirect('games'), null)
})

test('readPendingRedirect prefers the query parameter over the stored value', () => {
  const store = new Map<string, string>([['pending_redirect', '/winners']])
  globalThis.sessionStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, value),
    removeItem: (key: string) => void store.delete(key),
    clear: () => store.clear(),
    key: () => null,
    length: store.size,
  } as unknown as Storage

  try {
    assert.equal(readPendingRedirect('/wallet'), '/wallet')
    assert.equal(readPendingRedirect(null), '/winners')
    assert.equal(readPendingRedirect(''), '/winners')
  } finally {
    // @ts-expect-error - the test env has no sessionStorage of its own.
    delete globalThis.sessionStorage
  }
})
