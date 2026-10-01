---
name: run-app
description: Run UNCLI or its landing page and screenshot it in headless Edge, to see a UI change rendered. Use when asked to run, preview or screenshot the app or website, or to confirm a UI change works for real.
---

# Run and screenshot UNCLI

A visual check means: serve the page, point headless Edge at it, then Read
the PNG and actually look at it. No app window needed.

## The app

<!-- Fill in once the app exists: how to serve its UI in a browser (e.g.
`npx vite build && npx vite preview --port 4173 --strictPort` in the frontend
folder, plus any backend it needs on a spare port) and the URL to screenshot.
Never kill a process you did not start: the user may have the real app running. -->

## The website

```bash
node scripts/serve-website.mjs 5181     # background; 5180 may be the user's own
for i in $(seq 1 30); do curl -sf http://localhost:5181/ >/dev/null && break; sleep 1; done
```

## Screenshot with headless Edge

```bash
S="$SCRATCHPAD"    # any writable dir; use the session scratchpad
"/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" \
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
