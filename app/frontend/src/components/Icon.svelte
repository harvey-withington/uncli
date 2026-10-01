<script lang="ts" module>
  import ArrowUp from 'lucide-svelte/icons/arrow-up'
  import BarChart from 'lucide-svelte/icons/chart-no-axes-column'
  import Bookmark from 'lucide-svelte/icons/bookmark'
  import BookmarkCheck from 'lucide-svelte/icons/bookmark-check'
  import Briefcase from 'lucide-svelte/icons/briefcase'
  import Check from 'lucide-svelte/icons/check'
  import ChevronLeft from 'lucide-svelte/icons/chevron-left'
  import ChevronRight from 'lucide-svelte/icons/chevron-right'
  import ChevronsLeft from 'lucide-svelte/icons/chevrons-left'
  import ChevronsRight from 'lucide-svelte/icons/chevrons-right'
  import CircleAlert from 'lucide-svelte/icons/circle-alert'
  import Code from 'lucide-svelte/icons/code'
  import Copy from 'lucide-svelte/icons/copy'
  import Download from 'lucide-svelte/icons/download'
  import FolderOpen from 'lucide-svelte/icons/folder-open'
  import Gauge from 'lucide-svelte/icons/gauge'
  import Loader from 'lucide-svelte/icons/loader-circle'
  import LogIn from 'lucide-svelte/icons/log-in'
  import MessageCircle from 'lucide-svelte/icons/message-circle'
  import Microscope from 'lucide-svelte/icons/microscope'
  import Monitor from 'lucide-svelte/icons/monitor'
  import Moon from 'lucide-svelte/icons/moon'
  import Pencil from 'lucide-svelte/icons/pencil'
  import Plus from 'lucide-svelte/icons/plus'
  import Settings from 'lucide-svelte/icons/settings'
  import ShieldX from 'lucide-svelte/icons/shield-x'
  import Shrink from 'lucide-svelte/icons/shrink'
  import Sparkles from 'lucide-svelte/icons/sparkles'
  import Square from 'lucide-svelte/icons/square'
  import Sun from 'lucide-svelte/icons/sun'
  import Trash from 'lucide-svelte/icons/trash-2'
  import TriangleAlert from 'lucide-svelte/icons/triangle-alert'
  import Users from 'lucide-svelte/icons/users'
  import Wrench from 'lucide-svelte/icons/wrench'
  import X from 'lucide-svelte/icons/x'

  // Icon names as used in YAML config (Lucide's kebab-case), plus a few
  // the app uses itself. Unknown names fall back to a neutral glyph.
  const ICONS = {
    'arrow-up': ArrowUp, 'bar-chart': BarChart, bookmark: Bookmark, 'bookmark-check': BookmarkCheck,
    briefcase: Briefcase, check: Check, 'chevron-left': ChevronLeft, 'chevron-right': ChevronRight,
    'chevrons-left': ChevronsLeft, 'chevrons-right': ChevronsRight, 'circle-alert': CircleAlert, code: Code,
    copy: Copy, download: Download, 'folder-open': FolderOpen, gauge: Gauge, loader: Loader, 'log-in': LogIn,
    'message-circle': MessageCircle, microscope: Microscope, monitor: Monitor, moon: Moon, pencil: Pencil,
    plus: Plus, settings: Settings, 'shield-x': ShieldX, shrink: Shrink, sparkles: Sparkles, square: Square,
    sun: Sun, trash: Trash, 'triangle-alert': TriangleAlert, users: Users, wrench: Wrench, x: X,
  } as const

  export type IconName = keyof typeof ICONS
</script>

<script lang="ts">
  interface Props {
    name: string
    size?: number
    label?: string
    spin?: boolean
  }

  let { name, size = 16, label, spin = false }: Props = $props()
  const Cmp = $derived(ICONS[name as IconName] ?? Sparkles)
</script>

<span class="icon" class:spin role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  <Cmp {size} strokeWidth={1.75} />
</span>

<style>
  .icon {
    display: inline-flex;
    flex: none;
    line-height: 0;
  }
  .spin {
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
