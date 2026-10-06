// Artifacts for the mock's Chat session: two pages of versions (the
// itinerary becomes a printable table on page 2, the Lisbon plan), a
// Mermaid route, an SVG and a file no turn recorded, so the pane, paging
// back and every viewer can be seen.
import type { ArtifactContent, ArtifactFile, ArtifactRef, Page, TouchedFile } from './types'

const ITINERARY_V1 = `<!doctype html><html><head><title>Lisbon</title><style>
body{font-family:system-ui,sans-serif;margin:24px;color:#1d2433}h1{margin:0 0 8px}li{margin:4px 0}
</style></head><body><h1>Lisbon weekend</h1><ol><li>Saturday: Alfama, a tasca, Baixa</li><li>Sunday: Belém, LX Factory, Cais do Sodré</li></ol>
<p id="n"></p><script>document.getElementById('n').textContent = 'Scripts run in the sandbox: ' + (2 + 2)</script></body></html>`

const ITINERARY_V2 = `<!doctype html><html><head><title>Lisbon</title><style>
body{font-family:Georgia,serif;margin:24px;color:#1d2433}h1{margin:0 0 4px;color:#b3261e}table{border-collapse:collapse}td{border:1px solid #ccc;padding:4px 10px}
@media print{body{margin:0}}</style></head><body><h1>Lisbon weekend (printable)</h1>
<table><tr><td>Sat 09:00</td><td>Miradouro de Santa Luzia</td></tr><tr><td>Sat 13:00</td><td>Lunch in Alfama</td></tr>
<tr><td>Sun 10:00</td><td>Belém, pastéis de nata</td></tr><tr><td>Sun 17:00</td><td>Sunset at Cais do Sodré</td></tr></table></body></html>`

const ROUTE = `flowchart LR
  A[Alfama] --> B[Baixa]
  B --> C[Belém]
  C --> D[LX Factory]
  D --> E[Cais do Sodré]`

const BUDGET = `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="160" viewBox="0 0 320 160">
<rect x="20" y="30" width="180" height="26" fill="#1a73e8"/><text x="210" y="48" font-family="sans-serif" font-size="13">Food €70</text>
<rect x="20" y="66" width="40" height="26" fill="#e37400"/><text x="70" y="84" font-family="sans-serif" font-size="13">Transport €15</text>
<rect x="20" y="102" width="64" height="26" fill="#188038"/><text x="94" y="120" font-family="sans-serif" font-size="13">Entry €25</text></svg>`

const NOTES = `# Packing\n\n- Comfortable shoes\n- A light jacket for the evening\n`

const CONTENT: Record<string, string> = {
  'mock-itinerary-1': ITINERARY_V1, 'mock-itinerary-2': ITINERARY_V2, 'mock-route-1': ROUTE, 'mock-budget-2': BUDGET,
}
const LIVE: Record<string, string> = { 'notes.md': NOTES, 'itinerary.html': ITINERARY_V2, 'route.mmd': ROUTE, 'budget.svg': BUDGET }

const size = (s: string) => new TextEncoder().encode(s).length
const dir = 'C:\\Users\\you\\chat\\artifacts\\'

export const MOCK_CHAT_ARTIFACTS: { page1: Partial<Page>; page2: Partial<Page> } = {
  page1: {
    artifacts: [
      { path: 'itinerary.html', hash: 'mock-itinerary-1', size: size(ITINERARY_V1) },
      { path: 'route.mmd', hash: 'mock-route-1', size: size(ROUTE) },
    ],
    touchedFiles: [{ path: dir + 'itinerary.html', how: 'write' }, { path: dir + 'route.mmd', how: 'write' }] satisfies TouchedFile[],
  },
  page2: {
    artifacts: [
      { path: 'itinerary.html', hash: 'mock-itinerary-2', size: size(ITINERARY_V2) },
      { path: 'budget.svg', hash: 'mock-budget-2', size: size(BUDGET) },
    ],
    touchedFiles: [{ path: dir + 'itinerary.html', how: 'edit', line: 2, added: 6, removed: 4 }, { path: dir + 'budget.svg', how: 'write', added: 4 }] satisfies TouchedFile[],
  },
}

export function mockArtifactFiles(sessionId: string): ArtifactFile[] {
  return sessionId === 's-chat' ? Object.entries(LIVE).map(([path, text]) => ({ path, size: size(text) })) : []
}

export function mockReadArtifact(ref: ArtifactRef): ArtifactContent {
  const text = ref.hash ? CONTENT[ref.hash] : LIVE[ref.path ?? '']
  if (text === undefined) throw new Error('that artifact no longer exists')
  const bytes = new TextEncoder().encode(text)
  let bin = ''
  for (const b of bytes) bin += String.fromCharCode(b)
  return { data: btoa(bin), size: bytes.length }
}
