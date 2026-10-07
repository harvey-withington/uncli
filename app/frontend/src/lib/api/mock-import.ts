// Saved CLI conversations for the mock's import picker: one from the
// terminal, a deleted UNCLI chat, one already imported, and one whose
// folder is gone. Importing makes a session with a page per turn.
import type { Page, TranscriptEntry } from './types'

const HOUR = 3_600_000

export function mockTranscripts(now = Date.now()): TranscriptEntry[] {
  return [
    {
      id: 'tr-terminal', workdir: 'C:\\Users\\you\\code\\billing-service', started: now - 3 * HOUR, updated: now - 2 * HOUR,
      firstQuestion: 'Why do invoices round to the wrong cent when the tax rate has three decimals?', turns: 6, cliVersion: '2.1.285',
      folderGone: false, fromChat: false, profile: 'code',
    },
    {
      id: 'tr-chat', workdir: 'C:\\Users\\you\\AppData\\Roaming\\uncli\\scratch\\0f3c', started: now - 30 * HOUR, updated: now - 29 * HOUR,
      firstQuestion: 'Draft a polite note declining the Thursday meeting.', turns: 2, cliVersion: '2.1.285',
      folderGone: false, fromChat: true, profile: 'chat',
    },
    {
      id: 'tr-imported', workdir: 'C:\\Users\\you\\code', started: now - 50 * HOUR, updated: now - 48 * HOUR,
      firstQuestion: 'Why does TestMultiTurnPartial fail about one run in five?', turns: 1, cliVersion: '2.1.285',
      sessionId: 's-code', folderGone: false, fromChat: false, profile: 'code',
    },
    {
      id: 'tr-gone', workdir: 'C:\\Users\\you\\AppData\\Local\\Temp\\spike-41', started: now - 200 * HOUR, updated: now - 199 * HOUR,
      firstQuestion: 'Try the stream-json mode with two turns.', turns: 2, cliVersion: '2.1.280',
      folderGone: true, fromChat: false, profile: 'code',
    },
  ]
}

// mockTranscriptPages are the pages an imported mock conversation gets.
export function mockTranscriptPages(entry: TranscriptEntry, sessionId: string): Page[] {
  const pages: Page[] = []
  for (let i = 0; i < entry.turns; i++) {
    const at = entry.started + i * 600_000
    pages.push({
      id: `${sessionId}-p${i + 1}`, sessionId, seq: i + 1,
      question: i === 0 ? entry.firstQuestion : `Follow-up question ${i + 1}`,
      model: 'sonnet', modifiers: [], answerMd: i === 0 ? 'Imported from the terminal: the answer as the CLI saved it.' : 'Done.',
      trace: [], touchedFiles: [], status: 'done', bookmarked: false, pinned: false,
      inputTokens: 40, outputTokens: 600, cacheRead: 18000, cacheWrite: 900, costUsd: 0, durationMs: 9000, startedAt: at, finishedAt: at + 9000,
    })
  }
  return pages
}
