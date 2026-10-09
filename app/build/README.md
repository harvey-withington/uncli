# Build files

What `wails build` uses to package UNCLI. `bin/` is the output and isn't
versioned.

- `appicon.png`: the app icon. `windows/icon.ico` is made from it when
  missing.
- `windows/info.json`: the exe's version information, filled from
  `app/wails.json` (product name, version, company, copyright).
  `npm run release` stamps the release's version into `wails.json` for the
  build and puts it back afterwards.
- `windows/installer/project.nsi`: the NSIS installer script
  (`wails build -nsis`). It installs the WebView2 runtime if Windows lacks
  it. `wails_tools.nsh` beside it is generated on each build.
- `windows/wails.exe.manifest`: the Windows application manifest.
- `darwin/`: Wails' macOS defaults. UNCLI isn't built for macOS yet.

Delete a file here and `wails build` writes Wails' default back.
