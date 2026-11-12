import test from 'node:test'
import assert from 'node:assert/strict'
import {
  extractStartInviteCode,
  buildInviteUrl,
  buildWebInviteUrl,
  buildInviteMessage,
  shareViaTelegram,
  computeTelegramInsets,
} from './telegram.ts'

test('Telegram insets sum device safe area and Telegram content safe area', () => {
  const { safe, content, total } = computeTelegramInsets({
    safeAreaInset: { top: 44, bottom: 34, left: 0, right: 0 },
    contentSafeAreaInset: { top: 56, bottom: 0, left: 0, right: 0 },
  })
  assert.deepEqual(safe, { top: 44, bottom: 34, left: 0, right: 0 })
  assert.deepEqual(content, { top: 56, bottom: 0, left: 0, right: 0 })
  assert.deepEqual(total, { top: 100, bottom: 34, left: 0, right: 0 })
})

test('Telegram insets fall back to zero on older clients', () => {
  const { total } = computeTelegramInsets({})
  assert.deepEqual(total, { top: 0, bottom: 0, left: 0, right: 0 })
})

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

test('shareViaTelegram opens the share dialogue in a new tab', () => {
  const originalWindow = (globalThis as unknown as { window?: unknown }).window
  const calls: Array<{ url: string; target?: string }> = []
  const openedHandle = { opener: {} as unknown }
  ;(globalThis as unknown as { window?: unknown }).window = {
    open: (url: string, target?: string) => {
      calls.push({ url, target })
      return openedHandle
    },
  }

  shareViaTelegram('https://example.com/games?join=ABC123', 'play with me')

  assert.equal(calls.length, 1)
  assert.equal(calls[0].target, '_blank')
  assert.ok(calls[0].url.startsWith('https://t.me/share/url?'))
  assert.ok(calls[0].url.includes(encodeURIComponent('https://example.com/games?join=ABC123')))
  ;(globalThis as unknown as { window?: unknown }).window = originalWindow
})

test('shareViaTelegram does not navigate the Telegram Mini App away', () => {
  const originalWindow = (globalThis as unknown as { window?: unknown }).window
  const openLinkCalls: string[] = []
  const openTelegramLinkCalls: string[] = []
  ;(globalThis as unknown as { window?: unknown }).window = {
    // Simulate a blocked popup.
    open: () => null,
    Telegram: {
      WebApp: {
        openLink: (url: string) => openLinkCalls.push(url),
        openTelegramLink: (url: string) => openTelegramLinkCalls.push(url),
      },
    },
  }

  shareViaTelegram('https://example.com/games?join=ABC123', 'play with me')

  assert.equal(openTelegramLinkCalls.length, 0)
  assert.equal(openLinkCalls.length, 1)
  ;(globalThis as unknown as { window?: unknown }).window = originalWindow
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
