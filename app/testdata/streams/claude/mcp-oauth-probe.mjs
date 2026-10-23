// Re-recording the mcp-auth-* fixtures (no model turn, no usage): copy this
// file, mcp-oauth-server.mjs and mcp-oauth-share.mjs to a scratch folder,
// run `node mcp-oauth-server.mjs 47011` there, then `node mcp-oauth-probe.mjs
// <local|callback|custom|clear>`; the CLI gets its own config folder there,
// so nothing touches ~/.claude. Replace local paths with C:\Users\user\.claude
// before copying the recordings back.
//
// Drives the pinned CLI over stream-json through the MCP sign-in requests,
// with no model turn. Writes what goes in and out to <name>.in.jsonl and
// <name>.jsonl. Modes: local (follow the authUrl, letting the redirect hit
// the CLI's own listener), callback (catch the redirect ourselves and pass
// it with mcp_oauth_callback_url), clear (sign in, then mcp_clear_auth and
// mcp_reconnect).
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const [mode = 'local', name = `mcp-auth-${mode}`] = process.argv.slice(2)
const here = path.dirname(new URL(import.meta.url).pathname).replace(/^\/([A-Z]:)/, '$1')
const CLI = path.join(process.env.LOCALAPPDATA, 'uncli/cli/claude/2.1.285/claude.exe')
const cfgDir = fs.mkdtempSync(path.join(here, `cfg-${mode}-`))
const mcp = path.join(here, 'mcp.json')
fs.writeFileSync(mcp, JSON.stringify({ mcpServers: { locked: { type: 'http', url: 'http://127.0.0.1:47011/mcp' } } }))
const args = ['-p', '--input-format', 'stream-json', '--output-format', 'stream-json', '--verbose', '--model', 'haiku', '--mcp-config', mcp, '--strict-mcp-config', '--permission-prompt-tool', 'stdio']
fs.writeFileSync(path.join(here, `${name}.args.txt`), args.join(' ').replace(mcp, '../mcp-oauth.json'))
const env = { ...process.env, CLAUDE_CONFIG_DIR: cfgDir }
for (const k of Object.keys(env)) if (k.startsWith('CLAUDE_CODE_') || k === 'CLAUDECODE') delete env[k]
const cli = spawn(CLI, args, { env, stdio: ['pipe', 'pipe', 'pipe'] })
const inLog = fs.createWriteStream(path.join(here, `${name}.in.jsonl`))
const outLog = fs.createWriteStream(path.join(here, `${name}.jsonl`))
const waiting = new Map()
let n = 0
const send = req => {
  const id = `req_${++n}`
  const line = JSON.stringify({ type: 'control_request', request_id: id, request: req })
  inLog.write(line + '\n')
  cli.stdin.write(line + '\n')
  return new Promise(r => waiting.set(id, r))
}
let buf = ''
cli.stdout.on('data', d => {
  buf += d
  let i
  while ((i = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, i)
    buf = buf.slice(i + 1)
    if (!line.trim()) continue
    outLog.write(line + '\n')
    let m
    try { m = JSON.parse(line) } catch { continue }
    console.log('<<', line.slice(0, 400))
    if (m.type === 'control_response') waiting.get(m.response?.request_id)?.(m.response)
  }
})
cli.stderr.on('data', d => process.stderr.write('[cli] ' + d))
const sleep = ms => new Promise(r => setTimeout(r, ms))
const status = async () => {
  const r = await send({ subtype: 'mcp_status' })
  return (r.response?.mcpServers ?? []).map(s => `${s.name}:${s.status}:${(s.tools ?? []).length} tools`).join(', ')
}

await send({ subtype: 'initialize', hooks: null })
await sleep(1500)
console.log('STATUS before:', await status())
const auth = await send({ subtype: 'mcp_authenticate', serverName: 'locked', ...(mode === 'custom' ? { redirectUri: 'http://127.0.0.1:47099/cb' } : {}) })
console.log('AUTH:', JSON.stringify(auth))
const url = auth.response?.authUrl
if (url) {
  // The browser: follow /authorize, which redirects to the redirect URI.
  const r = await fetch(url, { redirect: 'manual' })
  const to = r.headers.get('location')
  console.log('REDIRECT:', to)
  if (mode === 'callback' || mode === 'custom') {
    const done = await send({ subtype: 'mcp_oauth_callback_url', serverName: 'locked', callbackUrl: to })
    console.log('CALLBACK:', JSON.stringify(done))
    await sleep(1000)
    console.log('RECONNECT after callback:', JSON.stringify(await send({ subtype: 'mcp_reconnect', serverName: 'locked' })))
  } else {
    const hit = await fetch(to).catch(e => ({ status: String(e) }))
    console.log('LISTENER said:', hit.status, typeof hit.text === 'function' ? (await hit.text()).slice(0, 200) : '')
  }
}
for (let i = 0; i < 10; i++) {
  await sleep(800)
  const s = await status()
  console.log('STATUS after:', s)
  if (s.includes(':connected')) break
}
if (mode === 'clear') {
  console.log('CLEAR:', JSON.stringify(await send({ subtype: 'mcp_clear_auth', serverName: 'locked' })))
  await sleep(500)
  console.log('STATUS after clear:', await status())
  console.log('RECONNECT:', JSON.stringify(await send({ subtype: 'mcp_reconnect', serverName: 'locked' })))
  await sleep(1000)
  console.log('STATUS after reconnect:', await status())
}
console.log('CONFIG DIR files:', fs.readdirSync(cfgDir, { recursive: true }).join(', '))
cli.stdin.end()
await sleep(500)
cli.kill()
inLog.end()
outLog.end()
