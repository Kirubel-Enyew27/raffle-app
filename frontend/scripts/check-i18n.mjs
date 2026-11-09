/**
 * Validates the English and Amharic dictionaries:
 *  - identical nested key structures (no missing/extra keys)
 *  - no empty values
 *  - matching {{placeholders}} for every key
 *  - valid JSON (parsing throws otherwise)
 *
 * Usage: npm run i18n:check
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const localesDir = join(root, 'src', 'lib', 'locales')

const flatten = (value, prefix = '', out = {}) => {
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (child && typeof child === 'object' && !Array.isArray(child)) flatten(child, path, out)
    else out[path] = child
  }
  return out
}

const placeholders = (value) =>
  [...String(value).matchAll(/{{\s*([\w.]+)\s*}}/g)]
    .map((match) => match[1])
    .sort()
    .join(',')

const en = flatten(JSON.parse(readFileSync(join(localesDir, 'en.json'), 'utf8')))
const am = flatten(JSON.parse(readFileSync(join(localesDir, 'am.json'), 'utf8')))

const problems = []
const enKeys = Object.keys(en)
const amKeys = Object.keys(am)

for (const key of enKeys) if (!(key in am)) problems.push(`missing in am.json: ${key}`)
for (const key of amKeys) if (!(key in en)) problems.push(`missing in en.json: ${key}`)

for (const [name, dict] of [['en.json', en], ['am.json', am]]) {
  for (const [key, value] of Object.entries(dict)) {
    if (typeof value !== 'string') problems.push(`${name}: ${key} is not a string`)
    else if (!value.trim()) problems.push(`${name}: ${key} is empty`)
  }
}

// Amharic may intentionally omit grammatically redundant variables (e.g. an
// explicit plural suffix), so only flag variables English requires that are
// missing from Amharic. The reverse is reported as a warning.
const INTENTIONAL_DIFFERENCES = new Set([
  // English renders "ticket(s)" through a `plural` suffix variable; Amharic
  // does not inflect the noun, so it only needs the count and cost.
  'raffles.buyButton',
])
const warnings = []
for (const key of enKeys) {
  if (!(key in am) || INTENTIONAL_DIFFERENCES.has(key)) continue
  const enVars = placeholders(en[key])
  const amVars = placeholders(am[key])
  if (enVars !== amVars) {
    const missing = enVars.split(',').filter((v) => v && !amVars.split(',').includes(v))
    const extra = amVars.split(',').filter((v) => v && !enVars.split(',').includes(v))
    if (missing.length) problems.push(`${key}: am.json is missing placeholder(s) ${missing.join(', ')}`)
    if (extra.length) warnings.push(`${key}: am.json has extra placeholder(s) ${extra.join(', ')}`)
  }
}

// Report dictionary keys that no source file mentions. These are usually stale
// copy left behind by a redesign; they are not an error.
const sources = []
const walk = (dir) => {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) walk(path)
    else if (/\.(ts|tsx)$/.test(entry)) sources.push(readFileSync(path, 'utf8'))
  }
}
walk(join(root, 'src'))
const blob = sources.join('\n')
const unused = enKeys.filter((key) => !blob.includes(key))

console.log(`en.json keys: ${enKeys.length}`)
console.log(`am.json keys: ${amKeys.length}`)

if (warnings.length) {
  console.log(`\nWarnings (${warnings.length}):`)
  for (const warning of warnings) console.log(`  - ${warning}`)
}

if (unused.length) {
  console.log(`\nUnused keys (${unused.length}) — prune when convenient, not an error:`)
  for (const key of unused.slice(0, 20)) console.log(`  - ${key}`)
  if (unused.length > 20) console.log(`  … ${unused.length - 20} more`)
}

if (problems.length) {
  console.error(`\nFAILED — ${problems.length} problem(s):`)
  for (const problem of problems) console.error(`  - ${problem}`)
  process.exit(1)
}

console.log('\nOK — dictionaries are structurally in sync.')
