// Two CLIs sharing one config folder (two sessions): sign in through A;
// does B, already running and needs-auth, connect on mcp_reconnect? And
// does a third CLI started afterwards connect at once?
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const here = path.dirname(new URL(import.meta.url).pathname).replace(/^\/([A-Z]:)/, '$1')
const CLI = path.join(process.env.LOCALAPPDATA, 'uncli/cli/claude/2.1.285/claude.exe')
const cfgDir = fs.mkdtempSync(path.join(here, 'cfg-share-'))
const mcp = path.join(here, 'mcp.json')
const args = ['-p', '--input-format', 'stream-json', '--output-format', 'stream-json', '--verbose', '--model', 'haiku', '--mcp-config', mcp, '--strict-mcp-config', '--permission-prompt-tool', 'stdio']
const env = { ...process.env, CLAUDE_CONFIG_DIR: cfgDir }
for (const k of Object.keys(env)) if (k.startsWith('CLAUDE_CODE_') || k === 'CLAUDECODE') delete env[k]
const sleep = ms => new Promise(r => setTimeout(r, ms))

function start(tag) {
  const cli = spawn(CLI, args, { env, stdio: ['pipe', 'pipe', 'ignore'] })
  const waiting = new Map()
  let n = 0
  let buf = ''
  cli.stdout.on('data', d => {
    buf += d
    let i
    while ((i = buf.indexOf('\n')) >= 0) {
      const line = buf.slice(0, i)
      buf = buf.slice(i + 1)
      try {
        const m = JSON.parse(line)
        if (m.type === 'control_response') waiting.get(m.response?.request_id)?.(m.response)
        else if (m.type === 'system') console.log(tag, 'system event:', line.slice(0, 200))
      } catch {}
    }
  })
  const send = req => {
    const id = `${tag}_${++n}`
    cli.stdin.write(JSON.stringify({ type: 'control_request', request_id: id, request: req }) + '\n')
    return new Promise(r => waiting.set(id, r))
  }
  const status = async () => (await send({ subtype: 'mcp_status' })).response?.mcpServers?.map(s => `${s.name}:${s.status}`).join(',')
  return { cli, send, status }
}

const A = start('A')
const B = start('B')
await A.send({ subtype: 'initialize', hooks: null })
await B.send({ subtype: 'initialize', hooks: null })
await sleep(1500)
console.log('A', await A.status(), 'B', await B.status())
const auth = await A.send({ subtype: 'mcp_authenticate', serverName: 'locked' })
const to = (await fetch(auth.response.authUrl, { redirect: 'manual' })).headers.get('location')
await fetch(to)
await sleep(2000)
console.log('after A signs in: A', await A.status(), 'B', await B.status())
console.log('B reconnect:', JSON.stringify(await B.send({ subtype: 'mcp_reconnect', serverName: 'locked' })))
await sleep(1000)
console.log('B after reconnect:', await B.status())
const C = start('C')
await C.send({ subtype: 'initialize', hooks: null })
await sleep(2000)
console.log('C, started after:', await C.status())
for (const x of [A, B, C]) { x.cli.stdin.end(); x.cli.kill() }
