// Builds the Windows installer and keeps a copy on a test server, stamped
// with the date and a sequence number (uncli-setup-2026-10-03-001.exe).
// Only the newest few builds are kept there; older ones are deleted.
//
//   node scripts/publish-build.mjs [--skip-build] [--keep 6]
//
// Only main makes the regular builds. On any other branch the build is
// published apart from them, by itself (--branch is still taken, and does
// nothing more): named uncli-<branch>-<date>-<n>.exe (feat/packages →
// uncli-feat-packages-2026-11-04-001.exe) and stamped <release>+<branch>.<n>,
// with its own sequence, and only that branch's builds are pruned. The
// regular builds are never counted or deleted by it.
//
// The server is reached over ssh/scp (OpenSSH, key auth, a cmd.exe shell
// on the far side). Its address stays out of the repo:
//   UNCLI_BUILD_HOST   host name
//   UNCLI_BUILD_USER   ssh user
//   UNCLI_BUILD_DIR    folder in that user's home (default uncli-builds)
//
// Prints one JSON line describing the build for the build log.

import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { readFileSync, statSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const args = process.argv.slice(2)
const skipBuild = args.includes('--skip-build')
// The branch being built; '' for main, whose builds are the regular ones.
const current = execFileSync('git', ['branch', '--show-current'], { encoding: 'utf8' }).trim()
if (!current) {
  console.error('Check out a branch first: a detached HEAD has no name to publish a build under.')
  process.exit(2)
}
const branch = current === 'main' ? '' : current.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
const prefix = branch ? `uncli-${branch}` : 'uncli-setup'
const keepAt = args.indexOf('--keep')
const keep = keepAt >= 0 ? Number(args[keepAt + 1]) : 6
const host = process.env.UNCLI_BUILD_HOST
const user = process.env.UNCLI_BUILD_USER
const dir = process.env.UNCLI_BUILD_DIR || 'uncli-builds'
if (!host || !user) {
  console.error('Set UNCLI_BUILD_HOST and UNCLI_BUILD_USER.')
  process.exit(2)
}
if (!(keep >= 1)) {
  console.error('--keep needs a number of builds, 1 or more.')
  process.exit(2)
}
const target = `${user}@${host}`

const appDir = fileURLToPath(new URL('../app/', import.meta.url))
const installer = join(appDir, 'build', 'bin', 'uncli-amd64-installer.exe')

const ssh = cmd => execFileSync('ssh', ['-o', 'BatchMode=yes', target, cmd], { encoding: 'utf8' })

// The builds already there, oldest first.
const pattern = new RegExp(`^${prefix}-(\\d{4}-\\d{2}-\\d{2})-(\\d{3,})\\.exe$`)
// The brackets matter: without them cmd makes `& dir …` part of the `if`,
// so the listing only ran when the folder was missing.
const listed = ssh(`(if not exist ${dir} mkdir ${dir}) & dir /b ${dir} 2>nul & exit 0`)
const builds = listed
  .split(/\r?\n/)
  .map(s => s.trim())
  .filter(s => pattern.test(s))
  .sort((a, b) => Number(a.match(pattern)[2]) - Number(b.match(pattern)[2]))

const last = builds.length ? Number(builds.at(-1).match(pattern)[2]) : 0
const seq = last + 1
const d = new Date()
const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
const name = `${prefix}-${date}-${String(seq).padStart(3, '0')}.exe`

// A test build carries the last release's version with +dev.<build>
// (0.3.0-alpha+dev.66). Build metadata counts for nothing when versions are
// compared, so it checks for updates as that release would: it's offered
// the next one, never an older one (decision 0025).
const lastTag = execFileSync('git', ['describe', '--tags', '--abbrev=0', '--match', 'v*'], { encoding: 'utf8' }).trim()
const version = `${lastTag.replace(/^v/, '')}+${branch || 'dev'}.${seq}`

// No -clean: it would delete build/bin, which fails while `npm run dev` runs.
// --skip-build keeps whatever build/bin holds, stamped or not. The
// installer and the exe's file version are stamped with the release's
// numbers (0.9.0), as release.mjs does, so Windows' installed apps list
// shows the version rather than wails.json's placeholder; wails.json is put
// back afterwards.
if (!skipBuild) {
  const wailsJson = join(appDir, 'wails.json')
  const original = readFileSync(wailsJson, 'utf8')
  const config = JSON.parse(original)
  config.info.productVersion = lastTag.replace(/^v/, '').match(/^\d+\.\d+\.\d+/)[0]
  writeFileSync(wailsJson, JSON.stringify(config, null, 2) + '\n')
  try {
    execFileSync('wails', ['build', '-nsis', '-platform', 'windows/amd64', '-ldflags', `"-X github.com/harvey-withington/uncli/app/internal/buildinfo.Version=${version}"`], { cwd: appDir, stdio: 'inherit', shell: true })
  } finally {
    writeFileSync(wailsJson, original)
  }
}

const data = readFileSync(installer)
const sha256 = createHash('sha256').update(data).digest('hex')
execFileSync('scp', ['-q', '-o', 'BatchMode=yes', installer, `${target}:${dir}/${name}`], { stdio: 'inherit' })

const all = [...builds, name]
const removed = all.slice(0, Math.max(0, all.length - keep))
if (removed.length) ssh(removed.map(f => `del /q ${dir}\\${f}`).join(' & '))

console.log(JSON.stringify({
  build: seq,
  name,
  version: skipBuild ? undefined : version,
  date,
  bytes: statSync(installer).size,
  sha256,
  path: `${dir}/${name}`,
  kept: all.slice(removed.length),
  removed,
}))
