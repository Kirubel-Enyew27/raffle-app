import test from 'node:test'
import assert from 'node:assert/strict'
import {
  extractStartInviteCode,
  buildInviteUrl,
  buildWebInviteUrl,
  buildInviteMessage,
} from './telegram.ts'

test('buildWebInviteUrl builds standard url with code', () => {
  const url = buildWebInviteUrl('abc9x2')
  assert.match(url, /\/games\?join=ABC9X2$/)
})

test('buildInviteUrl generates valid URL with code', () => {
  const url = buildInviteUrl('xy8k1p')
  assert.match(url, /XY8K1P/)
})

test('buildInviteMessage formats title, link, and code', () => {
  const msg = buildInviteMessage('Tic-Tac-Toe Blitz', 'ABC123', 'https://example.com/join')
  assert.ok(msg.includes('Tic-Tac-Toe Blitz'))
  assert.ok(msg.includes('ABC123'))
  assert.ok(msg.includes('https://example.com/join'))
})

test('extractStartInviteCode handles empty / undefined environment safely', () => {
  const code = extractStartInviteCode()
  assert.equal(code, null)
})

test('extractStartInviteCode extracts from window.Telegram.WebApp.initDataUnsafe', () => {
  const originalWindow = (globalThis as unknown as { window?: unknown }).window
  ;(globalThis as unknown as { window?: unknown }).window = {
    Telegram: {
      WebApp: {
        initDataUnsafe: {
          start_param: 'join_TEST99',
        },
      },
    },
    location: { href: 'https://example.com/' },
  }
  const code = extractStartInviteCode()
  assert.equal(code, 'TEST99')
  ;(globalThis as unknown as { window?: unknown }).window = originalWindow
})

test('extractStartInviteCode extracts from URL searchParams join', () => {
  const originalWindow = (globalThis as unknown as { window?: unknown }).window
  ;(globalThis as unknown as { window?: unknown }).window = {
    location: { href: 'https://example.com/games?join=ROOM42' },
  }
  const code = extractStartInviteCode()
  assert.equal(code, 'ROOM42')
  ;(globalThis as unknown as { window?: unknown }).window = originalWindow
})

test('extractStartInviteCode extracts from URL searchParams tgWebAppStartParam', () => {
  const originalWindow = (globalThis as unknown as { window?: unknown }).window
  ;(globalThis as unknown as { window?: unknown }).window = {
    location: { href: 'https://example.com/?tgWebAppStartParam=join_TGAPP1' },
  }
  const code = extractStartInviteCode()
  assert.equal(code, 'TGAPP1')
  ;(globalThis as unknown as { window?: unknown }).window = originalWindow
})
