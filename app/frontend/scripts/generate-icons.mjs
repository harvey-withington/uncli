// Generates every icon from the one mark (assets/svg/uncli-mark.svg): the
// app's favicon and PNG, the Windows .ico and the 1024 px app icon Wails
// builds from, and the website's icon. Skips work when nothing changed.
// Adapted from BRUV's frontend/scripts/generate-icons.mjs.
import { copyFileSync, mkdirSync, statSync, writeFileSync } from 'fs'
import { dirname, resolve } from 'path'
import { fileURLToPath } from 'url'
import sharp from 'sharp'
import pngToIco from 'png-to-ico'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = resolve(__dirname, '..') // app/frontend
const appRoot = resolve(root, '..') // app
const repoRoot = resolve(appRoot, '..')

const source = resolve(repoRoot, 'assets/svg/uncli-mark.svg')

const ICO_SIZES = [16, 32, 48, 64, 128, 256]

const targets = [
  { path: resolve(root, 'public/uncli-mark.svg'), type: 'copy' },
  { path: resolve(root, 'public/uncli-mark.png'), type: 'png', size: 256 },
  { path: resolve(root, 'public/icon.ico'), type: 'ico', sizes: ICO_SIZES },
  { path: resolve(appRoot, 'build/appicon.png'), type: 'png', size: 1024 },
  { path: resolve(appRoot, 'build/windows/icon.ico'), type: 'ico', sizes: ICO_SIZES },
  { path: resolve(repoRoot, 'website/icon.svg'), type: 'copy' },
  { path: resolve(repoRoot, 'website/icon-512.png'), type: 'png', size: 512 },
]

function isStale() {
  let srcTime
  try {
    srcTime = statSync(source).mtimeMs
  } catch {
    console.error(`Source not found: ${source}`)
    process.exit(1)
  }
  return targets.some(t => {
    try {
      return statSync(t.path).mtimeMs < srcTime
    } catch {
      return true
    }
  })
}

// Scale the mark up to fill the square, keeping its aspect ratio: trim the
// SVG's transparent margin, then fit the longer side to the edge (the mark
// is taller than wide, so only the sides stay transparent).
async function render(svgPath, size) {
  const trimmed = await sharp(svgPath, { density: 600 }).trim().png().toBuffer()
  return sharp(trimmed)
    .resize(size, size, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
}

async function generatePng(svgPath, outPath, size) {
  mkdirSync(dirname(outPath), { recursive: true })
  await (await render(svgPath, size)).toFile(outPath)
}

async function generateIco(svgPath, outPath, sizes) {
  mkdirSync(dirname(outPath), { recursive: true })
  const pngBuffers = await Promise.all(sizes.map(async size => (await render(svgPath, size)).toBuffer()))
  writeFileSync(outPath, await pngToIco(pngBuffers))
}

async function main() {
  if (!isStale() && !process.argv.includes('--force')) {
    console.log('Icons up to date, skipping.')
    return
  }

  console.log('Generating icons from SVG...')

  for (const target of targets) {
    switch (target.type) {
      case 'copy':
        mkdirSync(dirname(target.path), { recursive: true })
        copyFileSync(source, target.path)
        console.log(`  Copied → ${target.path}`)
        break
      case 'png':
        await generatePng(source, target.path, target.size)
        console.log(`  PNG ${target.size}×${target.size} → ${target.path}`)
        break
      case 'ico':
        await generateIco(source, target.path, target.sizes)
        console.log(`  ICO ${target.sizes.join(',')} → ${target.path}`)
        break
    }
  }

  console.log('Done.')
}

main().catch(err => {
  console.error(err)
  process.exit(1)
})
