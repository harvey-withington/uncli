<script lang="ts" module>
  import ArrowUp from 'lucide-svelte/icons/arrow-up'
  import BarChart from 'lucide-svelte/icons/chart-no-axes-column'
  import Bookmark from 'lucide-svelte/icons/bookmark'
  import BookmarkCheck from 'lucide-svelte/icons/bookmark-check'
  import Briefcase from 'lucide-svelte/icons/briefcase'
  import Check from 'lucide-svelte/icons/check'
  import ChevronDown from 'lucide-svelte/icons/chevron-down'
  import ChevronLeft from 'lucide-svelte/icons/chevron-left'
  import ChevronRight from 'lucide-svelte/icons/chevron-right'
  import ChevronUp from 'lucide-svelte/icons/chevron-up'
  import ChevronsLeft from 'lucide-svelte/icons/chevrons-left'
  import ChevronsRight from 'lucide-svelte/icons/chevrons-right'
  import CircleAlert from 'lucide-svelte/icons/circle-alert'
  import Code from 'lucide-svelte/icons/code'
  import Coffee from 'lucide-svelte/icons/coffee'
  import Copy from 'lucide-svelte/icons/copy'
  import Download from 'lucide-svelte/icons/download'
  import File from 'lucide-svelte/icons/file'
  import FileText from 'lucide-svelte/icons/file-text'
  import Flag from 'lucide-svelte/icons/flag'
  import FlaskConical from 'lucide-svelte/icons/flask-conical'
  import FastForward from 'lucide-svelte/icons/fast-forward'
  import FolderOpen from 'lucide-svelte/icons/folder-open'
  import Eye from 'lucide-svelte/icons/eye'
  import GlobeLock from 'lucide-svelte/icons/globe-lock'
  import Hand from 'lucide-svelte/icons/hand'
  import Zap from 'lucide-svelte/icons/zap'
  import Gauge from 'lucide-svelte/icons/gauge'
  import ImageIcon from 'lucide-svelte/icons/image'
  import Info from 'lucide-svelte/icons/info'
  import Lightbulb from 'lucide-svelte/icons/lightbulb'
  import ListOrdered from 'lucide-svelte/icons/list-ordered'
  import Loader from 'lucide-svelte/icons/loader-circle'
  import LogIn from 'lucide-svelte/icons/log-in'
  import MessageCircle from 'lucide-svelte/icons/message-circle'
  import MessageSquareText from 'lucide-svelte/icons/message-square-text'
  import Microscope from 'lucide-svelte/icons/microscope'
  import Monitor from 'lucide-svelte/icons/monitor'
  import Moon from 'lucide-svelte/icons/moon'
  import PanelRightClose from 'lucide-svelte/icons/panel-right-close'
  import PanelRightOpen from 'lucide-svelte/icons/panel-right-open'
  import Pencil from 'lucide-svelte/icons/pencil'
  import Play from 'lucide-svelte/icons/play'
  import Plus from 'lucide-svelte/icons/plus'
  import Rewind from 'lucide-svelte/icons/rewind'
  import ScanSearch from 'lucide-svelte/icons/scan-search'
  import Search from 'lucide-svelte/icons/search'
  import Shield from 'lucide-svelte/icons/shield'
  import Settings from 'lucide-svelte/icons/settings'
  import ShieldCheck from 'lucide-svelte/icons/shield-check'
  import ShieldX from 'lucide-svelte/icons/shield-x'
  import Shrink from 'lucide-svelte/icons/shrink'
  import Sparkles from 'lucide-svelte/icons/sparkles'
  import Square from 'lucide-svelte/icons/square'
  import Sun from 'lucide-svelte/icons/sun'
  import Table from 'lucide-svelte/icons/table'
  import Trash from 'lucide-svelte/icons/trash-2'
  import TriangleAlert from 'lucide-svelte/icons/triangle-alert'
  import Users from 'lucide-svelte/icons/users'
  import Wrench from 'lucide-svelte/icons/wrench'
  import X from 'lucide-svelte/icons/x'

  // Icon names as used in YAML config (Lucide's kebab-case), plus a few
  // the app uses itself. Unknown names fall back to a neutral glyph.
  const ICONS = {
    'arrow-up': ArrowUp, 'bar-chart': BarChart, bookmark: Bookmark, 'bookmark-check': BookmarkCheck,
    briefcase: Briefcase, check: Check, 'chevron-down': ChevronDown, 'chevron-up': ChevronUp, 'chevron-left': ChevronLeft, 'chevron-right': ChevronRight,
    'chevrons-left': ChevronsLeft, 'chevrons-right': ChevronsRight, 'circle-alert': CircleAlert, code: Code, coffee: Coffee,
    copy: Copy, download: Download, file: File, 'file-text': FileText, image: ImageIcon, 'folder-open': FolderOpen, gauge: Gauge, loader: Loader, 'log-in': LogIn,
    'message-circle': MessageCircle, microscope: Microscope, monitor: Monitor, moon: Moon, 'panel-right-close': PanelRightClose, 'panel-right-open': PanelRightOpen, pencil: Pencil,
    plus: Plus, play: Play, rewind: Rewind, search: Search, shield: Shield, 'fast-forward': FastForward, settings: Settings, 'shield-check': ShieldCheck, 'shield-x': ShieldX, shrink: Shrink, sparkles: Sparkles, square: Square,
    sun: Sun, table: Table, trash: Trash, flag: Flag, 'flask-conical': FlaskConical, info: Info,
    lightbulb: Lightbulb, 'list-ordered': ListOrdered, 'message-square-text': MessageSquareText, 'scan-search': ScanSearch, 'triangle-alert': TriangleAlert, users: Users, wrench: Wrench, x: X,
    eye: Eye, 'globe-lock': GlobeLock, hand: Hand, zap: Zap,
  } as const

  export type IconName = keyof typeof ICONS
</script>

<script lang="ts">
  interface Props {
    name: string
    size?: number
    label?: string
    spin?: boolean
    fill?: boolean // solid shapes (triangles, a set bookmark) instead of outlines
    flip?: boolean // mirrored left to right (a left-pointing play triangle)
  }

  let { name, size = 16, label, spin = false, fill = false, flip = false }: Props = $props()
  const Cmp = $derived(ICONS[name as IconName] ?? Sparkles)
</script>

<span class="icon" class:spin class:flip role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : 'true'}>
  <Cmp {size} strokeWidth={fill ? 1.5 : 1.75} fill={fill ? 'currentColor' : 'none'} />
</span>

<style>
  .icon {
    display: inline-flex;
    flex: none;
    line-height: 0;
  }
  .flip {
    transform: scaleX(-1);
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
