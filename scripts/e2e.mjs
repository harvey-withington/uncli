// End-to-end check of the Phase 1 definition of done, on Windows, against
// the real app and the real Claude CLI (your sign-in, small Haiku turns).
// It runs `wails dev`, drives the UI that serves in headless Edge, and
// inspects the real processes for console windows and leftovers. Results
// and screenshots go to <temp>/uncli-e2e. Usage: npm run test:e2e
import { spawn, execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import puppeteer from 'puppeteer-core'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const HERE = path.join(os.tmpdir(), 'uncli-e2e')
const DATA = path.join(HERE, 'data')
const SHOTS = path.join(HERE, 'shots')
fs.mkdirSync(HERE, { recursive: true })
const results = []
const ok = (name, pass, detail = '') => { results.push({ name, pass, detail }); console.log(`${pass ? 'PASS' : 'FAIL'}  ${name}${detail ? '  — ' + detail : ''}`) }
const sleep = ms => new Promise(r => setTimeout(r, ms))

function ps(cmd) {
  return execFileSync('powershell', ['-NoProfile', '-Command', cmd], { encoding: 'utf8' }).trim()
}
async function recordCopies(page) {
  await page.evaluate(() => {
    window.__copied = []
    const rec = t => { window.__copied.push(t); return Promise.resolve() }
    try { Object.defineProperty(navigator, 'clipboard', { value: { writeText: rec }, configurable: true }) } catch {}
    if (window.runtime) window.runtime.ClipboardSetText = rec
  })
}
const lastCopy = page => page.evaluate(() => window.__copied.at(-1) ?? '')
async function clipboard() {
  for (let i = 0; i < 10; i++) {
    try { return ps('Get-Clipboard -Raw') } catch { await sleep(300) }
  }
  return ''
}
function uncliCLIs() {
  const out = ps(`Get-Process claude -ErrorAction SilentlyContinue | Where-Object { $_.Path -like '*\\uncli\\cli\\*' } | ForEach-Object { "$($_.Id) $($_.MainWindowHandle)" }`)
  return out ? out.split(/\r?\n/).map(l => { const [id, h] = l.trim().split(' '); return { id: +id, window: h !== '0' } }) : []
}
function visibleConsoles() {
  // Any visible top-level window belonging to UNCLI's CLIs or console hosts they spawned.
  return ps(`Get-Process conhost,cmd,claude,powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 -and $_.StartTime -gt (Get-Date).AddMinutes(-15) } | ForEach-Object { "$($_.ProcessName) $($_.Id) $($_.MainWindowTitle)" }`)
}

const APPDIR = path.join(ROOT, 'app')
const EDGE = process.env.EDGE_PATH ?? 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const WAILS = process.env.WAILS_PATH ?? path.join(os.homedir(), 'go', 'bin', 'wails.exe')
const DEV = 'http://localhost:34115'

// wails dev runs the real app (window and Go backend) and serves the same
// UI, bound to that backend, at localhost:34115; production WebView2
// windows can't be driven over CDP because Wails clears the debug args.
async function launch() {
  const env = { ...process.env, UNCLI_DATA_DIR: DATA }
  for (const k of Object.keys(env)) if (k.startsWith('CLAUDE_CODE_') || k === 'CLAUDECODE') delete env[k]
  const proc = spawn(WAILS, ['dev', '-noreload', '-nocolour', '-skipbindings', '-devserver', 'localhost:34115'], { cwd: APPDIR, env, stdio: ['ignore', fs.openSync(path.join(HERE, 'wails-dev.log'), 'a'), fs.openSync(path.join(HERE, 'wails-dev.log'), 'a')] })
  for (let i = 0; i < 240; i++) {
    try { const r = await fetch(DEV + '/'); if (r.ok) break } catch {}
    await sleep(500)
  }
  await sleep(1500)
  const browser = await puppeteer.launch({ executablePath: EDGE, headless: true, args: [`--user-data-dir=${path.join(HERE, 'edge-' + Date.now())}`, '--window-size=1280,820'] })
  const page = await browser.newPage()
  await page.setViewport({ width: 1280, height: 820 })
  page.on('pageerror', e => console.log('PAGEERROR', String(e?.stack ?? e).slice(0, 900)))
  page.on('console', m => { if (m.type() === 'error' || m.type() === 'warn') console.log('CONSOLE', m.type(), m.text().slice(0, 300)) })
  await page.goto(DEV, { waitUntil: 'networkidle0', timeout: 60000 })
  return { proc, browser, page }
}

function appPid() {
  try { return ps(`Get-Process uncli-dev -ErrorAction SilentlyContinue | Where-Object { $_.Path -like '*UNCLI-1.0*app*build*bin*' } | Select-Object -First 1 -ExpandProperty Id`) } catch { return '' }
}

async function quit(proc, browser) {
  try { await browser.close() } catch {}
  // WM_CLOSE to the app window, like clicking its close button, so the
  // app's own shutdown runs; then stop the dev tooling.
  const pid = appPid()
  if (pid) { try { execFileSync('taskkill', ['/PID', pid]) } catch {} }
  for (let i = 0; i < 40 && appPid(); i++) await sleep(250)
  const exited = !appPid()
  try { execFileSync('taskkill', ['/PID', String(proc.pid), '/T', '/F']) } catch {}
  await sleep(500)
  return exited ? 0 : 'still running'
}

const text = page => page.evaluate(() => document.body.innerText)
async function waitText(page, s, timeout = 120000) {
  await page.waitForFunction(t => document.body.innerText.includes(t), { timeout, polling: 200 }, s)
}
async function shot(page, name) {
  fs.mkdirSync(SHOTS, { recursive: true })
  await page.screenshot({ path: path.join(SHOTS, name + '.png') })
}
async function clickText(page, selector, s) {
  const el = await page.waitForFunction((sel, t) => [...document.querySelectorAll(sel)].find(e => e.textContent.includes(t)), { timeout: 15000 }, selector, s)
  await el.click()
}
async function sendTurn(page, q) {
  const box = await page.waitForSelector('textarea')
  await box.click()
  await box.type(q)
  await page.keyboard.press('Enter')
}
async function waitIdle(page, timeout = 180000) {
  await sleep(400)
  await page.waitForFunction(() => !document.querySelector('.composer .stop') && !document.querySelector('.composer textarea')?.placeholder.startsWith('Answering'), { timeout, polling: 250 })
}
async function openSession(page, profileId) {
  const id = await page.evaluate(p => window.__uncli.sessions.find(s => s.profileId === p)?.id, profileId)
  const idx = await page.evaluate(id => window.__uncli.sessions.findIndex(s => s.id === id), id)
  const lis = await page.$$('.sidebar li .main')
  await lis[idx].click()
  await page.waitForFunction(id => window.__uncli.currentId === id, {}, id)
  await sleep(150)
  return id
}
const dumpStore = page => page.evaluate(() => { const a = window.__uncli; return Object.fromEntries(a.sessions.map(s => [s.profileId, { idx: a.index[s.id], cur: a.currentId === s.id, pages: (a.pages[s.id] ?? null)?.map(p => `${p.seq}:${p.id.slice(0, 8)}:${p.status}:${p.answerMd.slice(0, 24)}`), live: a.live[s.id] ? a.live[s.id].seq + ':' + a.live[s.id].text.slice(0, 20) : null }])) })
const bridge = (page, method, ...args) => page.evaluate((m, a) => window.go.bridge.App[m](...a), method, args)

fs.rmSync(DATA, { recursive: true, force: true })
const repo = path.join(HERE, 'repo')
const docs = path.join(HERE, 'docs')
fs.rmSync(repo, { recursive: true, force: true }); fs.rmSync(docs, { recursive: true, force: true })
fs.mkdirSync(repo, { recursive: true }); fs.mkdirSync(docs, { recursive: true })
execFileSync('git', ['-C', repo, 'init', '-q'])
fs.writeFileSync(path.join(docs, 'notes.md'), '# Notes\n\nThe launch date is 14 March.\n')

let { proc, browser, page } = await launch()
try {
  await waitText(page, 'Welcome to UNCLI', 30000)
  await shot(page, '01-welcome')
  ok('app starts signed in with the pinned CLI', (await text(page)).includes('Claude CLI 2.1.285'))

  // Chat session through the dialog.
  await clickText(page, 'button', 'New session')
  await waitText(page, 'Start session')
  await page.select('.dialog select', 'haiku')
  await shot(page, '02-new-session')
  await clickText(page, 'button', 'Start session')
  await waitText(page, 'Start a conversation')

  // Co-work and code sessions (the folder picker is a native dialog, so
  // these go through the same bridge call the dialog makes).
  const co = await bridge(page, 'CreateSession', 'cowork', docs, 'haiku')
  const code = await bridge(page, 'CreateSession', 'code', repo, 'haiku')
  await sleep(300)
  const sessions = await page.$$eval('.sidebar li', ls => ls.length)
  ok('three sessions listed', sessions === 3, `${sessions} in sidebar`)

  // Send in all three without waiting.
  const turns = { chat: 'Remember the word banjo. Reply with just OK.', cowork: 'What is the launch date in notes.md? Five words or fewer.', code: 'Run git status and say in one sentence whether there are any commits.' }
  for (const [profile, q] of Object.entries(turns)) {
    await openSession(page, profile)
    await sendTurn(page, q)
    await sleep(150)
  }
  await sleep(600)
  const busyLabels = await page.$$eval('.sidebar .badge .label', ls => ls.map(l => l.textContent))
  await shot(page, '03-three-busy')
  ok('three sessions run at once with live states', busyLabels.filter(l => ['Starting', 'Thinking', 'Writing', 'Running tools'].includes(l)).length >= 2, busyLabels.join(', '))

  const clis = uncliCLIs()
  ok('a CLI process for every session', clis.length >= 3, `${clis.length} processes (the CLI may start helpers)`)
  ok('no CLI process has a window', clis.every(c => !c.window))
  const consoles = visibleConsoles()
  ok('no console windows appeared', consoles === '', consoles)

  for (const profile of ['chat', 'cowork', 'code']) {
    await openSession(page, profile)
    await waitIdle(page)
  }
  await sleep(500)
  const states = await page.$$eval('.sidebar .badge .label', ls => ls.map(l => l.textContent))
  ok('all sessions finished', !states.some(s => ['Starting', 'Thinking', 'Writing', 'Running tools', 'Error'].includes(s)), states.join(', ') || 'all idle')

  // Code session shows its trace.
  let lis
  await openSession(page, 'code')
  await waitText(page, 'tool call')
  await clickText(page, '.trace .summary', 'tool call')
  await sleep(300)
  await shot(page, '04-code-trace')
  ok('code page shows the git status tool call', (await text(page)).includes('git status'))

  // Chat: model switch, then a code block to copy.
  await openSession(page, 'chat')
  await waitText(page, 'Page 1 of 1')
  await page.select('.toolbar select', 'sonnet')
  await sleep(300)
  await sendTurn(page, 'What word did I ask you to remember? Then show a fenced python code block that prints that word, and nothing else.')
  await waitText(page, 'Page 2 of 2')
  await waitIdle(page)
  await sleep(1500)
  const t2 = (await text(page)).toLowerCase()
  ok('model switch keeps the conversation', t2.includes('banjo'))
  const chips = await page.$$eval('.page .chip.model', cs => cs.map(c => c.textContent.trim()))
  ok('page chip shows the new model', chips.some(c => /sonnet/i.test(c)), chips.join(', '))
  await recordCopies(page)
  await page.hover('.code')
  await page.click('.code .copy')
  await sleep(300)
  const clip = await lastCopy(page)
  ok('copy code puts only the code on the clipboard', /print\(/.test(clip) && !clip.includes('```'), JSON.stringify(clip.slice(0, 60)))
  const mdBlock = await page.$('.answer .block')
  if (mdBlock) {
    await mdBlock.hover()
    await page.click('.answer .block .copy')
    await sleep(300)
    const mdClip = await lastCopy(page)
    ok('copy markdown copies source markdown', mdClip.length > 0, JSON.stringify(mdClip.slice(0, 60)))
  }
  await page.hover('.question')
  await page.click('.q-copy .copy')
  await sleep(300)
  ok('copy question copies the question', (await lastCopy(page)).startsWith('What word did I ask you to remember?'))
  await shot(page, '05-model-switch')

  // Keyboard: back, bookmark, forward.
  await page.click('.scroll')
  await page.keyboard.press('ArrowLeft')
  await waitText(page, 'Page 1 of 2', 5000)
  await page.keyboard.press('b')
  await page.waitForSelector('.bm.on', { timeout: 5000 })
  await page.keyboard.press('ArrowRight')
  await waitText(page, 'Page 2 of 2', 5000)
  ok('arrow keys page and B bookmarks', true)

  // Efficiency mode in co-work.
  await openSession(page, 'cowork')
  await waitText(page, 'Page 1 of 1')
  await clickText(page, '.toolbar button', 'Efficiency Mode')
  await page.waitForSelector('.toolbar .toggle.on')
  await sendTurn(page, 'Explain what a hash map is.')
  await waitText(page, 'Page 2 of 2')
  await waitIdle(page)
  const lean = await page.$eval('.answer', a => a.innerText.length)
  const leanChips = await page.$$eval('.page .chip.mod', cs => cs.map(c => c.textContent.trim()))
  await clickText(page, '.toolbar button', 'Efficiency Mode')
  await sleep(300)
  await sendTurn(page, 'Explain what a hash map is.')
  await waitText(page, 'Page 3 of 3')
  await waitIdle(page)
  const full = await page.$eval('.answer', a => a.innerText.length)
  const fullChips = await page.$$eval('.page .chip.mod', cs => cs.map(c => c.textContent.trim()))
  await shot(page, '06-efficiency-off')
  ok('Efficiency Mode changes the answer and chips; off is honoured', lean < full && leanChips.includes('Efficiency Mode') && !fullChips.includes('Efficiency Mode'), `on ${lean} chars ${leanChips}, off ${full} chars ${fullChips}`)

  // Interrupt a turn that streams into the answer.
  await openSession(page, 'code')
  await sendTurn(page, 'In your reply (not a file, no tools), count from 1 to 300 with one number per line.')
  await page.waitForSelector('.composer .stop', { timeout: 30000 })
  await page.waitForFunction(() => document.querySelector('.answer')?.innerText.length > 40, { timeout: 60000 })
  await page.click('.composer .stop')
  await waitText(page, 'Stopped before the answer finished.', 20000)
  await shot(page, '07-interrupted')
  ok('stop interrupts the turn', true)

  const pagesBefore = { chat: 2, co: 4 }
  const exit = await quit(proc, browser)
  await sleep(1000)
  const left = uncliCLIs()
  ok('quitting stops every CLI process', left.length === 0, `exit ${exit}, ${left.length} left`)

  ;({ proc, browser, page } = await launch())
  await page.waitForSelector('.sidebar li', { timeout: 30000 })
  const restored = await page.$$eval('.sidebar li .title', ts => ts.map(t => t.textContent))
  ok('relaunch restores every session', restored.length === 3, restored.join(' | '))
  await page.waitForFunction(() => window.__uncli?.ready)
  await openSession(page, 'chat')
  await waitText(page, `Page ${pagesBefore.chat} of ${pagesBefore.chat}`)
  await page.click('.scroll')
  await page.keyboard.press('ArrowLeft')
  await page.waitForSelector('.bm.on', { timeout: 5000 })
  ok('bookmarks survive a relaunch', true)
  await sendTurn(page, 'What word did I ask you to remember at the very start? One word.')
  await waitText(page, `Page ${pagesBefore.chat + 1} of ${pagesBefore.chat + 1}`)
  await waitIdle(page)
  await sleep(500)
  ok('the conversation continues after relaunch', (await page.$eval('.answer', a => a.innerText)).toLowerCase().includes('banjo'))
  await shot(page, '08-after-relaunch')
  // Light theme shot
  await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'))
  await shot(page, '09-light')
} catch (e) {
  ok('e2e run', false, String(e?.stack ?? e).split(/\r?\n/).slice(0, 3).join(' | '))
  try { console.log('store', JSON.stringify(await dumpStore(page), null, 1)) } catch {}
  try { await shot(page, 'zz-failure') } catch {}
} finally {
  try { await quit(proc, browser) } catch {}
  await sleep(1000)
  ok('no CLI processes left behind', uncliCLIs().length === 0)
  fs.writeFileSync(path.join(HERE, 'results.json'), JSON.stringify(results, null, 2))
  const failed = results.filter(r => !r.pass).length
  console.log(`\n${results.length - failed} passed, ${failed} failed`)
  process.exit(failed ? 1 : 0)
}
