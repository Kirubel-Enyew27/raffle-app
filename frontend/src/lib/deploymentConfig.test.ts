import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const config = JSON.parse(
  readFileSync(new URL('../../vercel.json', import.meta.url), 'utf8'),
) as {
  rewrites: Array<{ source: string; destination: string }>
  headers: Array<{ source: string; headers: Array<{ key: string; value: string }> }>
}

test('SPA fallback matches routes but never missing hashed assets', () => {
  const source = config.rewrites[0]?.source
  assert.equal(typeof source, 'string')

  const matcher = new RegExp(`^${source}$`)
  assert.equal(matcher.test('/'), true)
  assert.equal(matcher.test('/admin/live'), true)
  assert.equal(matcher.test('/games/match/42'), true)
  assert.equal(matcher.test('/assets/AdminLivePage-F60NY5hl.js'), false)
  assert.equal(matcher.test('/assets/AdminLivePage-F60NY5hl.css'), false)
  assert.equal(matcher.test('/brand/kwik-icon-192.png'), false)
})

test('HTML and SPA routes revalidate while content-hashed assets are immutable', () => {
  const assetHeaders = config.headers.find(header => header.source === '/assets/(.*)')
  assert.deepEqual(assetHeaders?.headers, [
    { key: 'Cache-Control', value: 'public, max-age=31536000, immutable' },
  ])

  const htmlHeaders = config.headers.find(header => header.source === '/((?!assets/).*)')
  assert.deepEqual(htmlHeaders?.headers, [{ key: 'Cache-Control', value: 'no-cache' }])
})
