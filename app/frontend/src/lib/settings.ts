// Settings is split into tabs. Each section has an id (#settings-<id>) that
// palette commands and buttons elsewhere open Settings at; the section
// decides the tab. The last tab is remembered per viewer.

export type SettingsTab = 'general' | 'approvals' | 'providers' | 'containers'

export const SETTINGS_TABS: SettingsTab[] = ['general', 'approvals', 'providers', 'containers']

// Which tab each section is on.
const SECTION_TAB: Record<string, SettingsTab> = {
  appearance: 'general',
  notify: 'general',
  editor: 'general',
  quick: 'general',
  safe: 'approvals',
  decider: 'approvals',
  provider: 'providers',
  cli: 'providers',
  account: 'providers',
  'provider-containers': 'providers',
  containers: 'containers',
}

export function tabOf(section: string): SettingsTab | null {
  return SECTION_TAB[section] ?? null
}

// tabsFor is the tabs shown: Containers only where they run (Windows).
export function tabsFor(platform: string | undefined): SettingsTab[] {
  return SETTINGS_TABS.filter(t => t !== 'containers' || platform === 'windows')
}

const KEY = 'uncli-settings-tab'

export function loadSettingsTab(shown: SettingsTab[]): SettingsTab {
  try {
    const v = localStorage.getItem(KEY) as SettingsTab | null
    if (v && shown.includes(v)) return v
  } catch {
    // storage unavailable: start at the first tab
  }
  return shown[0] ?? 'general'
}

export function saveSettingsTab(tab: SettingsTab) {
  try {
    localStorage.setItem(KEY, tab)
  } catch {
    // not remembered; fine
  }
}

// nextTab is where an arrow key, Home or End moves in a vertical tab list.
export function nextTab(tabs: SettingsTab[], current: SettingsTab, key: string): SettingsTab | null {
  const i = tabs.indexOf(current)
  if (i < 0 || tabs.length === 0) return null
  switch (key) {
    case 'ArrowDown': return tabs[(i + 1) % tabs.length] ?? null
    case 'ArrowUp': return tabs[(i - 1 + tabs.length) % tabs.length] ?? null
    case 'Home': return tabs[0] ?? null
    case 'End': return tabs[tabs.length - 1] ?? null
  }
  return null
}
