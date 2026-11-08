// Sets the version of UNCLI's packages (@uncli/client and @uncli/chat,
// decision 0026), which move with UNCLI's own: run it while preparing a
// release, with the changelog. @uncli/chat depends on exactly the same
// version of @uncli/client. The release (scripts/release.mjs) stops if the
// packages don't carry the version being released.
//
//   npm run set-version -- 1.0.0-alpha
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const packages = join(root, 'app', 'frontend', 'packages')
const version = (process.argv[2] || '').replace(/^v/, '')
if (!/^\d+\.\d+\.\d+([-.]?[0-9A-Za-z.-]+)?$/.test(version)) {
  console.error(`Give a version like 1.0.0 or 1.0.0-alpha, not "${version}".`)
  process.exit(2)
}

for (const name of ['client', 'chat']) {
  const path = join(packages, name, 'package.json')
  const pkg = JSON.parse(readFileSync(path, 'utf8'))
  pkg.version = version
  if (pkg.dependencies?.['@uncli/client']) pkg.dependencies['@uncli/client'] = version
  writeFileSync(path, JSON.stringify(pkg, null, 2) + '\n')
}
// The workspace's lockfile records the packages' versions too.
execFileSync('npm', ['install', '--package-lock-only', '--ignore-scripts'], { cwd: join(root, 'app', 'frontend'), stdio: 'inherit', shell: true })
console.log(`@uncli/client and @uncli/chat are ${version}.`)
