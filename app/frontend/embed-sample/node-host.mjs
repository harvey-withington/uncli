// A Node host running its own UNCLI (decision 0027), with nothing but
// Node's own modules: it starts uncli-core with a key it makes, serves the
// sample page, and forwards /uncli/ to the core, adding the key. A real
// host puts its own sign-in in front of /uncli/.
//
//   npm --prefix app/frontend run build:embed
//   go build -o uncli-core.exe ./app/cmd/uncli-core
//   node app/frontend/embed-sample/node-host.mjs --core ./uncli-core.exe --data <folder>
//
// then open the address it prints.
import { spawn } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { createReadStream, existsSync, statSync } from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { createInterface } from 'node:readline'
import { parseArgs } from 'node:util'

const { values: opt } = parseArgs({
  options: {
    core: { type: 'string', default: 'uncli-core' },
    data: { type: 'string' },
    page: { type: 'string', default: path.join(import.meta.dirname, '..', 'dist-embed', 'sample') },
    port: { type: 'string', default: '8791' },
    'user-cli-config': { type: 'boolean', default: false },
  },
})
if (!opt.data) throw new Error('give a folder for the host’s UNCLI with --data')
const origin = `http://127.0.0.1:${opt.port}`

// The core: a key only this host knows, and a pipe on its stdin, so it
// ends when this process does.
const key = randomBytes(32).toString('hex')
const args = ['--data', opt.data, '--origin', origin, '--name', 'Node sample host']
if (opt['user-cli-config']) args.push('--user-cli-config')
const core = spawn(opt.core, args, { env: { ...process.env, UNCLI_CORE_KEY: key }, stdio: ['pipe', 'pipe', 'inherit'] })
const { port: corePort } = await new Promise((resolve, reject) => {
  createInterface({ input: core.stdout }).once('line', line => resolve(JSON.parse(line)))
  core.once('exit', code => reject(new Error(`uncli-core exited (${code})`)))
})

// /uncli/… goes to the core, with the key added; the event stream passes
// through as it comes.
function forward(req, res) {
  const headers = { ...req.headers, host: `127.0.0.1:${corePort}`, authorization: `Bearer ${key}` }
  const up = http.request({ host: '127.0.0.1', port: corePort, method: req.method, path: req.url.slice('/uncli'.length) || '/', headers }, r => {
    res.writeHead(r.statusCode ?? 502, r.headers)
    r.pipe(res)
  })
  up.on('error', () => res.writeHead(502).end())
  req.pipe(up)
}

const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' }
function serveFile(req, res) {
  const rel = decodeURIComponent(new URL(req.url, origin).pathname)
  const file = path.join(opt.page, rel.endsWith('/') ? rel + 'index.html' : rel)
  if (!file.startsWith(path.resolve(opt.page)) || !existsSync(file) || !statSync(file).isFile()) return res.writeHead(404).end()
  res.writeHead(200, { 'Content-Type': types[path.extname(file)] ?? 'application/octet-stream' })
  createReadStream(file).pipe(res)
}

http.createServer((req, res) => (req.url.startsWith('/uncli/') ? forward(req, res) : serveFile(req, res))).listen(Number(opt.port), '127.0.0.1', () => {
  console.log(`Open ${origin}/?src=/uncli`)
})
