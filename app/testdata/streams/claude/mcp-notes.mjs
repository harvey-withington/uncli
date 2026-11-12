// A small MCP server over stdio for the fixtures that need MCP tools with
// annotations (mcp-status-tool-hints, set-permission-mode-live): a read
// tool, a destructive one, a read one that reaches the web, and one with
// no hints. Each call answers "ok: <tool>". mcp.json starts it.
import { createInterface } from 'node:readline'

const tools = [
  { name: 'read_note', description: 'Read a note.', annotations: { readOnlyHint: true } },
  { name: 'delete_note', description: 'Delete a note.', annotations: { destructiveHint: true } },
  { name: 'lookup_web', description: 'Look something up on the web.', annotations: { readOnlyHint: true, openWorldHint: true } },
  { name: 'touch_note', description: 'Mark a note as seen.', annotations: {} },
].map(t => ({ ...t, inputSchema: { type: 'object', properties: { name: { type: 'string' } }, required: ['name'] } }))

const send = msg => process.stdout.write(JSON.stringify({ jsonrpc: '2.0', ...msg }) + '\n')

createInterface({ input: process.stdin }).on('line', line => {
  let req
  try {
    req = JSON.parse(line)
  } catch {
    return
  }
  if (req.id === undefined) return // a notification
  switch (req.method) {
    case 'initialize':
      return send({ id: req.id, result: { protocolVersion: req.params?.protocolVersion ?? '2025-06-18', capabilities: { tools: {} }, serverInfo: { name: 'notes', version: '1.0.0' } } })
    case 'tools/list':
      return send({ id: req.id, result: { tools } })
    case 'tools/call':
      return send({ id: req.id, result: { content: [{ type: 'text', text: `ok: ${req.params?.name}` }] } })
    default:
      return send({ id: req.id, error: { code: -32601, message: `no method ${req.method}` } })
  }
})
