// A server's pairing link, https://<machine>.<tailnet>.ts.net:<port>/m/#pair=<code>
// (decision 0022), read as Go's remote.ParsePairLink reads it: Settings
// checks a pasted link before pairing, and the mock pairs with it; null
// when it isn't one.
export function parsePairLink(link: string): { base: string; code: string } | null {
  let u: URL
  try {
    u = new URL(link.trim())
  } catch {
    return null
  }
  const code = new URLSearchParams(u.hash.replace(/^#/, '')).get('pair')?.trim().toUpperCase() ?? ''
  if (u.protocol !== 'https:' || !u.host || !code) return null
  return { base: `https://${u.host}`, code }
}
