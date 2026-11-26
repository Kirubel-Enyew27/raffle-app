/**
 * Verifies the production artifact is complete before it can be deployed.
 *
 * Vite writes a manifest listing every emitted JS/CSS chunk. The deploy step
 * must not publish an index.html that references a file missing from dist,
 * otherwise users can request a chunk that returns HTML or 404. Run through
 * `npm run build`; also fails if the build did not produce a manifest.
 */
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const dist = join(root, 'dist')
const manifestPath = join(dist, '.vite', 'manifest.json')
const indexPath = join(dist, 'index.html')

const problems = []

if (!existsSync(indexPath)) problems.push('dist/index.html is missing')
if (!existsSync(manifestPath)) problems.push('dist/.vite/manifest.json is missing (enable build.manifest)')

if (problems.length === 0) {
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
  const entries = Object.entries(manifest)

  for (const [key, entry] of entries) {
    for (const reference of [...(entry.imports ?? []), ...(entry.dynamicImports ?? [])]) {
      if (!manifest[reference]) {
        problems.push(`${key} references missing manifest entry ${reference}`)
      }
    }

    const files = [entry.file, ...(entry.css ?? []), ...(entry.assets ?? [])]
    for (const file of files) {
      if (file && !existsSync(join(dist, file))) {
        problems.push(`${key} references missing build file ${file}`)
      }
    }
  }

  const html = readFileSync(indexPath, 'utf8')
  for (const match of html.matchAll(/(?:src|href)="([^"]+\.(?:js|css))"/g)) {
    const reference = match[1]
    if (!reference.startsWith('/')) continue
    if (!existsSync(join(dist, reference.slice(1)))) {
      problems.push(`index.html references missing build file ${reference}`)
    }
  }

  // Every page module must have its own emitted chunk. This catches a route
  // that was accidentally turned into a bare import or omitted from the build.
  const pagesDir = join(root, 'src', 'pages')
  const pageFiles = []
  const walk = (dir) => {
    for (const entry of readdirSync(dir)) {
      const path = join(dir, entry)
      if (statSync(path).isDirectory()) walk(path)
      else if (/\.tsx?$/.test(entry)) pageFiles.push(path)
    }
  }
  walk(pagesDir)
  for (const page of pageFiles) {
    const key = relative(root, page).replaceAll('\\', '/')
    if (!manifest[key]) problems.push(`page module ${key} was not emitted as a build chunk`)
  }

  if (problems.length === 0) {
    console.log(
      `Build assets verified: ${entries.length} manifest entries, ${pageFiles.length} page chunks and index.html references exist.`,
    )
  }
}

if (problems.length) {
  console.error(`FAILED — incomplete production build (${problems.length} problem(s)):`)
  for (const problem of problems) console.error(`  - ${problem}`)
  process.exit(1)
}
