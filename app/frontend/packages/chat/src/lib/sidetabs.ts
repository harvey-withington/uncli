// The side panel's tabs (decision 0026): UNCLI's own, On this page
// (outline), Artifacts and File, and an embedding host's own, whose content
// the host puts in the slot side-tab-<id>. The app shows all of UNCLI's; an
// embedded chat shows what its host lists in host.sideTabs, or On this page
// and Artifacts. An empty list: no side panel at all.

export type BuiltinTab = 'outline' | 'artifacts' | 'file'

// HostSideTab is a host's own tab: its name and a UNCLI icon; its content
// is the host's, in the element's side-tab-<id> slot.
export interface HostSideTab {
  id: string
  label: string
  icon?: string
}

export type SideTab = BuiltinTab | HostSideTab

export const BUILTIN_TABS: BuiltinTab[] = ['outline', 'artifacts', 'file']
export const EMBEDDED_TABS: SideTab[] = ['outline', 'artifacts']

// A host tab's key among the panel's tabs, as remembered with the layout.
export const hostKey = (id: string) => `host:${id}` as const
export const hostIdOf = (key: string) => (key.startsWith('host:') ? key.slice(5) : null)
