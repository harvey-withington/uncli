// Serves website/ on http://localhost:5180 for previewing the landing page.
//
//   node scripts/serve-website.mjs [port]
//
// Zero dependencies; loopback only; no caching, so a browser refresh always
// shows the latest edit.

import { createServer } from 'node:http'
import { readFile, stat } from 'node:fs/promises'
import { extname, join, normalize, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(fileURLToPath(new URL('../website/', import.meta.url)))
const port = Number(process.argv[2] || process.env.PORT || 5180)

const types = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.webp': 'image/webp',
  '.woff2': 'font/woff2',
}

createServer(async (req, res) => {
  try {
    const path = decodeURIComponent(new URL(req.url, 'http://x').pathname)
    let file = normalize(join(root, path))
    if (file !== root && !file.startsWith(root + sep)) {
      res.writeHead(403).end('forbidden')
      return
    }
    if ((await stat(file)).isDirectory()) file = join(file, 'index.html')
    const body = await readFile(file)
    res.writeHead(200, {
      'Content-Type': types[extname(file).toLowerCase()] ?? 'application/octet-stream',
      'Cache-Control': 'no-store',
    })
    res.end(body)
  } catch {
    res.writeHead(404, { 'Content-Type': 'text/plain' }).end('not found')
  }
}).listen(port, '127.0.0.1', () => {
  console.log(`Serving website/ at http://localhost:${port}`)
})
