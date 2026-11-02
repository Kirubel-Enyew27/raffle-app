import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildGoogleStartUrl,
  googleErrorKey,
  googleLinkErrorKey,
  parseGoogleCallbackHash,
  sanitizeRedirect,
} from './google.ts'

test('sanitizeRedirect keeps internal paths and drops everything else', () => {
  assert.equal(sanitizeRedirect('/games/match/42'), '/games/match/42')
  assert.equal(sanitizeRedirect('  /wallet  '), '/wallet')
  assert.equal(sanitizeRedirect('//evil.example.com'), null)
  assert.equal(sanitizeRedirect('/\\evil.example.com'), null)
  assert.equal(sanitizeRedirect('https://evil.example.com'), null)
  assert.equal(sanitizeRedirect('games'), null)
  assert.equal(sanitizeRedirect(''), null)
  assert.equal(sanitizeRedirect(null), null)
})

test('buildGoogleStartUrl targets the backend and preserves a safe redirect', () => {
  const url = buildGoogleStartUrl('/api/v1', '/games/match/42')
  assert.equal(url, '/api/v1/auth/google?redirect=%2Fgames%2Fmatch%2F42')
})

test('buildGoogleStartUrl trims trailing slashes and drops unsafe redirects', () => {
  assert.equal(buildGoogleStartUrl('https://api.example.com/api/v1/', null), 'https://api.example.com/api/v1/auth/google')
  assert.equal(
    buildGoogleStartUrl('https://api.example.com/api/v1/', '//evil.example.com'),
    'https://api.example.com/api/v1/auth/google',
  )
})

test('parseGoogleCallbackHash extracts the one-time code and redirect', () => {
  const parsed = parseGoogleCallbackHash('#code=abc123&redirect=%2Fwallet')
  assert.equal(parsed.code, 'abc123')
  assert.equal(parsed.redirect, '/wallet')
  assert.equal(parsed.error, null)
})

test('parseGoogleCallbackHash ignores an unsafe redirect', () => {
  const parsed = parseGoogleCallbackHash('#code=abc123&redirect=https%3A%2F%2Fevil.example.com')
  assert.equal(parsed.code, 'abc123')
  assert.equal(parsed.redirect, null)
})

test('parseGoogleCallbackHash reports an empty fragment instead of throwing', () => {
  const parsed = parseGoogleCallbackHash('')
  assert.deepEqual(parsed, { code: null, redirect: null, error: null })
})

test('googleErrorKey maps known reasons and falls back for unknown ones', () => {
  assert.equal(googleErrorKey('access_denied'), 'auth.googleCancelled')
  assert.equal(googleErrorKey('account_exists'), 'auth.googleAccountExists')
  assert.equal(googleErrorKey('email_unverified'), 'auth.googleEmailUnverified')
  assert.equal(googleErrorKey('invalid_state'), 'auth.googleExpired')
  assert.equal(googleErrorKey('something-new'), 'auth.googleSignInFailed')
  assert.equal(googleErrorKey(null), 'auth.googleSignInFailed')
})

test('googleLinkErrorKey maps link outcomes and never falls back to a sign-in key', () => {
  assert.equal(googleLinkErrorKey('link_email_mismatch'), 'profile.googleLinkEmailMismatch')
  assert.equal(googleLinkErrorKey('link_conflict'), 'profile.googleLinkConflict')
  assert.equal(googleLinkErrorKey('already_linked'), 'profile.googleLinkAlreadyLinked')
  assert.equal(googleLinkErrorKey('account_disabled'), 'profile.googleLinkDisabled')
  assert.equal(googleLinkErrorKey('unknown-reason'), 'profile.googleLinkFailed')
  assert.equal(googleLinkErrorKey(null), 'profile.googleLinkFailed')
})
