// Builds a release into dist/ for the Release workflow to publish:
//
//   dist/uncli-<version>-windows-x64-setup.exe   the installer
//   dist/SHA256SUMS.txt                          its checksum
//   dist/LICENSE.txt, dist/THIRD-PARTY-NOTICES.md
//   release-notes.md                             CHANGELOG.md's section for this version
//
//   RELEASE_VERSION=1.0.0 node scripts/release.mjs     (or pass the version as an argument)
//
// The version is a tag without its v: 1.0.0, or a pre-release such as
// 1.0.0-rc1 or 1.0.0b1. Windows and NSIS only take numbers, so the installer
// and the exe are stamped with the numeric part (1.0.0); wails.json is put
// back afterwards. CHANGELOG.md must have a "## <version>" (or "## <x.y.z>")
// section, and THIRD-PARTY-NOTICES.md must be current (npm run notices).

import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { copyFileSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const appDir = join(root, 'app')
const dist = join(root, 'dist')

const version = (process.argv[2] || process.env.RELEASE_VERSION || '').replace(/^v/, '')
const m = version.match(/^(\d+\.\d+\.\d+)([-.]?[0-9A-Za-z.-]+)?$/)
if (!m) {
  console.error(`Give a version like 1.0.0 or 1.0.0-rc1 (RELEASE_VERSION or an argument), not "${version}".`)
  process.exit(2)
}
const numeric = m[1]

// The CHANGELOG section for a version: the lines after its "## " heading, up
// to the next one.
function section(lines, v) {
  const start = lines.findIndex(l => l.trim() === `## ${v}` || l.startsWith(`## ${v} `))
  if (start < 0) return undefined
  const end = lines.findIndex((l, i) => i > start && l.startsWith('## '))
  return lines.slice(start + 1, end < 0 ? undefined : end).join('\n').trim()
}
const changelog = readFileSync(join(root, 'CHANGELOG.md'), 'utf8').replace(/\r\n/g, '\n').split('\n')
const notes = section(changelog, version) ?? section(changelog, numeric)
if (!notes) {
  console.error(`CHANGELOG.md has no "## ${version}" or "## ${numeric}" section.`)
  process.exit(1)
}

execFileSync(process.execPath, [join(root, 'scripts', 'notices.mjs'), '--check'], { stdio: 'inherit' })

const wailsJson = join(appDir, 'wails.json')
const original = readFileSync(wailsJson, 'utf8')
const config = JSON.parse(original)
config.info.productVersion = numeric
writeFileSync(wailsJson, JSON.stringify(config, null, 2) + '\n')
try {
  execFileSync('wails', ['build', '-clean', '-nsis', '-platform', 'windows/amd64'], { cwd: appDir, stdio: 'inherit', shell: true })
} finally {
  writeFileSync(wailsJson, original)
}

rmSync(dist, { recursive: true, force: true })
mkdirSync(dist)
const setup = `uncli-${version}-windows-x64-setup.exe`
copyFileSync(join(appDir, 'build', 'bin', 'uncli-amd64-installer.exe'), join(dist, setup))
copyFileSync(join(root, 'LICENSE'), join(dist, 'LICENSE.txt'))
copyFileSync(join(root, 'THIRD-PARTY-NOTICES.md'), join(dist, 'THIRD-PARTY-NOTICES.md'))
const sha = createHash('sha256').update(readFileSync(join(dist, setup))).digest('hex')
writeFileSync(join(dist, 'SHA256SUMS.txt'), `${sha}  ${setup}\n`)

const body = notes +`\n\n**SHA-256** of \`${setup}\`: \`${sha}\`\n`
writeFileSync(join(root, 'release-notes.md'), body)

console.log(`Release ${version}: dist/${setup} (${sha}), notes in release-notes.md.`)
