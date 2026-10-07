// A sample theme file for the browser mock: ?themefile=sample applies it,
// to see a restyled UNCLI without a config folder.
import type { ThemeFileInfo } from './types'

export function mockTheme(): ThemeFileInfo {
  const info: ThemeFileInfo = { path: 'C:\\Users\\user\\AppData\\Roaming\\uncli\\theme.yaml', found: false }
  if (new URLSearchParams(location.search).get('themefile') !== 'sample') return info
  return {
    ...info,
    found: true,
    light: {
      bg: '#eef2f4', surface: '#ffffff', 'surface-2': '#e6ecef', 'surface-3': '#dbe3e7',
      text: '#0f2530', 'text-muted': '#4f6772', 'text-faint': '#87a0aa',
      border: '#d5dee2', 'border-strong': '#bccbd1',
      accent: '#0f766e', 'accent-hover': '#115e59', 'accent-text': '#ffffff', 'chart-bar': '#0f766e',
      radius: '4px', 'radius-sm': '3px', 'radius-lg': '6px',
    },
    dark: {
      bg: '#0b1418', surface: '#111d22', 'surface-2': '#17262c', 'surface-3': '#1e3038',
      text: '#e3eef1', 'text-muted': '#93aab2', 'text-faint': '#62797f',
      border: '#1f3138', 'border-strong': '#2c434b',
      accent: '#2dd4bf', 'accent-hover': '#5eead4', 'accent-text': '#0b1418', 'chart-bar': '#2dd4bf',
      radius: '4px', 'radius-sm': '3px', 'radius-lg': '6px',
    },
  }
}
