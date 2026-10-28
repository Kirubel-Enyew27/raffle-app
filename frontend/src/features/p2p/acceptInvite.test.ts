import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseInviteError } from './inviteError.ts'

test('parseInviteError correctly handles 404 / NOT_FOUND', () => {
  const err404 = { status: 404, message: 'Request failed with status code 404' }
  const result = parseInviteError(err404, 'ABC9X2')
  assert.equal(result.title, 'Challenge Not Found')
  assert.ok(result.description.includes('ABC9X2'))
  assert.equal(result.canRetry, false)
  assert.equal(result.isExpired, false)

  const errNotFoundCode = { code: 'NOT_FOUND', message: 'match not found' }
  const result2 = parseInviteError(errNotFoundCode, 'XYZ123')
  assert.equal(result2.title, 'Challenge Not Found')
  assert.ok(result2.description.includes('XYZ123'))
})

test('parseInviteError correctly handles 410 / INVITATION_EXPIRED', () => {
  const err410 = { status: 410, message: 'This invitation has expired' }
  const result = parseInviteError(err410, 'ABC9X2')
  assert.equal(result.title, 'Challenge Expired')
  assert.equal(result.isExpired, true)
  assert.equal(result.canRetry, false)

  const errExpiredCode = { code: 'INVITATION_EXPIRED', message: 'expired' }
  const result2 = parseInviteError(errExpiredCode, 'ABC9X2')
  assert.equal(result2.title, 'Challenge Expired')
  assert.equal(result2.isExpired, true)
})

test('parseInviteError correctly handles 403 / FORBIDDEN', () => {
  const err403 = { status: 403, code: 'FORBIDDEN', message: 'Forbidden' }
  const result = parseInviteError(err403, 'ABC9X2')
  assert.equal(result.title, 'Access Restricted')
  assert.equal(result.canRetry, false)
})

test('parseInviteError correctly handles 409 / MATCH_FULL', () => {
  const errFull = { status: 409, code: 'MATCH_FULL', message: 'match is already full' }
  const result = parseInviteError(errFull, 'ABC9X2')
  assert.equal(result.title, 'Challenge Full')
  assert.equal(result.canRetry, false)
})

test('parseInviteError correctly handles 409 / CANNOT_JOIN_OWN_MATCH', () => {
  const errOwn = { status: 409, code: 'CANNOT_JOIN_OWN_MATCH', message: 'cannot join your own match' }
  const result = parseInviteError(errOwn, 'ABC9X2')
  assert.equal(result.title, 'Your Challenge')
  assert.equal(result.canRetry, false)
})

test('parseInviteError correctly handles network errors', () => {
  const netErr = { message: 'Network Error', status: undefined }
  const result = parseInviteError(netErr, 'ABC9X2')
  assert.equal(result.title, 'Connection Error')
  assert.equal(result.canRetry, true)
})

test('parseInviteError handles unexpected server 500 errors', () => {
  const err500 = { status: 500, message: 'internal server error' }
  const result = parseInviteError(err500, 'ABC9X2')
  assert.equal(result.title, 'Unable to Load Challenge')
  assert.equal(result.canRetry, true)
})
