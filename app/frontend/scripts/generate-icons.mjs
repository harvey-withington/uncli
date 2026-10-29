// Generates every icon from the one mark (assets/svg/uncli-mark.svg): the
// app's favicon and PNG, the Windows .ico and the 1024 px app icon Wails
// builds from, the phone app's icons (m/public) and the website's icon. Skips work when nothing changed.
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
  { path: resolve(root, 'm/public/icon-192.png'), type: 'png', size: 192 },
  { path: resolve(root, 'm/public/icon-512.png'), type: 'png', size: 512 },
  { path: resolve(root, 'm/public/icon-maskable-512.png'), type: 'maskable', size: 512 },
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

// A maskable icon (Android's adaptive icons crop it to a circle or
// squircle, keeping at least the middle circle 80% across): the mark,
// which is round, at 78% so it fills that circle, on the phone app's
// background (its manifest's background_color).
const MASK_BACKGROUND = { r: 0x18, g: 0x18, b: 0x1b, alpha: 1 }

async function generateMaskable(svgPath, outPath, size) {
  mkdirSync(dirname(outPath), { recursive: true })
  const inner = Math.round(size * 0.78)
  const mark = await (await render(svgPath, inner)).toBuffer()
  await sharp({ create: { width: size, height: size, channels: 4, background: MASK_BACKGROUND } })
    .composite([{ input: mark, gravity: 'center' }])
    .png()
    .toFile(outPath)
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
      case 'maskable':
        await generateMaskable(source, target.path, target.size)
        console.log(`  Maskable ${target.size}×${target.size} → ${target.path}`)
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
