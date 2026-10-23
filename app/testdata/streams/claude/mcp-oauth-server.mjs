// A fake MCP server behind OAuth (streamable HTTP), for the sign-in spike.
// Discovery, dynamic client registration, authorize (auto-approves with a
// 302 to the redirect URI), token, and /mcp which needs the bearer token.
import http from 'node:http'

const port = Number(process.argv[2] ?? 47011)
const base = `http://127.0.0.1:${port}`
const TOKEN = 'tok-123'
const log = (...a) => console.error('[server]', ...a)

const json = (res, code, body, headers = {}) => {
  res.writeHead(code, { 'content-type': 'application/json', ...headers })
  res.end(JSON.stringify(body))
}

http.createServer(async (req, res) => {
  const url = new URL(req.url, base)
  let body = ''
  for await (const c of req) body += c
  log(req.method, url.pathname + url.search, req.headers.authorization ? '(bearer)' : '')
  if (url.pathname.startsWith('/.well-known/oauth-protected-resource')) {
    return json(res, 200, { resource: `${base}/mcp`, authorization_servers: [base] })
  }
  if (url.pathname.startsWith('/.well-known/oauth-authorization-server') || url.pathname.startsWith('/.well-known/openid-configuration')) {
    return json(res, 200, {
      issuer: base, authorization_endpoint: `${base}/authorize`, token_endpoint: `${base}/token`, registration_endpoint: `${base}/register`,
      response_types_supported: ['code'], grant_types_supported: ['authorization_code', 'refresh_token'],
      code_challenge_methods_supported: ['S256'], token_endpoint_auth_methods_supported: ['none'],
    })
  }
  if (url.pathname === '/register') {
    const r = JSON.parse(body || '{}')
    log('register redirect_uris', r.redirect_uris)
    return json(res, 201, { client_id: 'spike-client', redirect_uris: r.redirect_uris, token_endpoint_auth_method: 'none', grant_types: ['authorization_code', 'refresh_token'], response_types: ['code'] })
  }
  if (url.pathname === '/authorize') {
    const to = new URL(url.searchParams.get('redirect_uri'))
    to.searchParams.set('code', 'code-abc')
    to.searchParams.set('state', url.searchParams.get('state') ?? '')
    res.writeHead(302, { location: to.toString() })
    return res.end()
  }
  if (url.pathname === '/token') {
    log('token', body)
    return json(res, 200, { access_token: TOKEN, token_type: 'Bearer', expires_in: 3600, refresh_token: 'refresh-1' })
  }
  if (url.pathname === '/mcp') {
    if (req.headers.authorization !== `Bearer ${TOKEN}`) {
      return json(res, 401, { error: 'unauthorized' }, { 'www-authenticate': `Bearer resource_metadata="${base}/.well-known/oauth-protected-resource"` })
    }
    if (req.method !== 'POST') return json(res, 405, {})
    const m = JSON.parse(body)
    if (m.id === undefined) { res.writeHead(202); return res.end() }
    const reply = result => json(res, 200, { jsonrpc: '2.0', id: m.id, result })
    if (m.method === 'initialize') return reply({ protocolVersion: m.params?.protocolVersion ?? '2025-06-18', capabilities: { tools: {} }, serverInfo: { name: 'locked', version: '1.0.0' } })
    if (m.method === 'tools/list') return reply({ tools: [{ name: 'secret', description: 'Says the secret', inputSchema: { type: 'object', properties: {} }, annotations: { readOnlyHint: true } }] })
    if (m.method === 'tools/call') return reply({ content: [{ type: 'text', text: 'the secret is 42' }] })
    return json(res, 200, { jsonrpc: '2.0', id: m.id, error: { code: -32601, message: 'no' } })
  }
  json(res, 404, {})
}).listen(port, '127.0.0.1', () => log('listening', base))
