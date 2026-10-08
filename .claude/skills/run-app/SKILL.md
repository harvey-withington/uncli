---
name: run-app
description: Run UNCLI or its landing page and screenshot it in headless Edge, to see a UI change rendered. Use when asked to run, preview or screenshot the app or website, or to confirm a UI change works for real.
---

# Run and screenshot UNCLI

A visual check means: serve the page, point headless Edge at it, then Read
the PNG and actually look at it. No app window needed.

## The app

The UI runs in a plain browser on a mock backend (canned sessions, streamed
answers), so most visual checks need no Go and no CLI:

```bash
cd app/frontend
npx vite build && (npx vite preview --port 4317 --strictPort &)   # background; pick a spare port
for i in $(seq 1 30); do curl -sf http://localhost:4317/ >/dev/null && break; sleep 1; done
```

Screenshot `http://localhost:4317/`. Query parameters pick a scenario:
`?mock=setup` (first-run download), `?mock=signin`, `?mock=empty`
(welcome screen); `?theme=light` or `?theme=dark` forces a theme. To open a
dialog or click through, drive the page with puppeteer-core (a root dev
dependency) pointed at Edge, as `scripts/e2e.mjs` does.

To see the real app with the real CLI, `npm run test:e2e` runs `wails dev`,
drives the UI it serves at `http://localhost:34115` (bound to the real Go
backend) and writes screenshots to `%TEMP%\uncli-e2e\shots`. It uses the
user's Claude sign-in and a little of their usage, so run it when a change
touches the backend or the CLI, not for every UI tweak. The production
WebView2 window can't be driven over CDP: Wails clears the debug arguments.

Never kill a process you did not start: the user may have the real app running.

## The website

```bash
node scripts/serve-website.mjs 5181     # background; 5180 may be the user's own
for i in $(seq 1 30); do curl -sf http://localhost:5181/ >/dev/null && break; sleep 1; done
```

## Screenshot with headless Edge

```bash
S="$SCRATCHPAD"    # any writable dir; use the session scratchpad
# The versioned binary: the top-level msedge.exe is a launcher (see below).
EDGE=$(ls -d "/c/Program Files (x86)/Microsoft/Edge/Application/"[0-9]*/msedge.exe | sort -V | tail -1)
"$EDGE" \
  --headless=new --disable-gpu --no-first-run --no-default-browser-check \
  --user-data-dir="$S\\edge-profile-$RANDOM" \
  --window-size=1440,900 --hide-scrollbars --virtual-time-budget=5000 \
  --screenshot="$S\\page.png" \
  "http://localhost:5181/"
```

Gotchas:

- Pass Windows-style paths (`C:\...`) to `--screenshot` and `--user-data-dir`;
  a `/c/...` path exits 0 and writes nothing.
- Use a fresh `--user-data-dir` every run. A lingering headless instance on a
  reused profile absorbs the launch: exit 0, no file.
- Since Edge 154, `msedge.exe` (even the versioned one) hands off to another
  process and exits at once. A one-shot `--screenshot` still works, but
  `puppeteer.launch` takes the exit for a failed start and leaves a headless
  Edge behind each time. To drive Edge, start it yourself with
  `--remote-debugging-port=<port>` and a fresh `--user-data-dir`, then
  `puppeteer.connect({ browserURL: 'http://127.0.0.1:<port>' })`, as
  `scripts/e2e.mjs` does (`startEdge`).
- `--virtual-time-budget` lets fonts, fetches and sockets settle first.
- The capture follows the system theme; tokens must cover light and dark.
- Headless Edge will not go narrower than about 500 CSS px. For a phone
  layout, screenshot a wrapper page holding a 390px-wide iframe.

## Stop what you started

Kill by port, and only your own ports:

```bash
pid=$(netstat -ano | grep LISTENING | grep ":5181 " | awk '{print $NF}' | head -1)
[ -n "$pid" ] && taskkill //PID $pid //T //F
```
