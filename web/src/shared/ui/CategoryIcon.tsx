import {
  Monitor,
  Wrench,
  ShoppingCart,
  Truck,
  PenTool,
  HardHat,
  Users,
  Building2,
  ShieldCheck,
  Lightbulb,
  Wallet,
  Headset,
  FileText,
  Folder,
  Workflow,
  MonitorCog,
  Package,
  DraftingCompass,
  Handshake,
  CircleHelp,
  ArrowLeftRight,
  Eye,
  Database,
  GraduationCap,
  BookOpen,
  Plug,
  FileSignature,
  ListChecks,
  Timer,
  MessagesSquare,
  FolderKanban,
  CalendarRange,
  Target,
  Archive,
  Trophy,
  type LucideIcon,
} from 'lucide-react';

// Explicit allow-list instead of importing every lucide icon (keeps the bundle small).
// Must contain every icon name used by the backend seed (backend/internal/seed/data.go);
// unknown names fall back to a folder.
const ICONS: Record<string, LucideIcon> = {
  // dataset categories (CR1)
  'arrow-left-right': ArrowLeftRight,
  eye: Eye,
  database: Database,
  'graduation-cap': GraduationCap,
  'file-text': FileText,
  'book-open': BookOpen,
  plug: Plug,
  users: Users,
  'file-signature': FileSignature,
  'list-checks': ListChecks,
  timer: Timer,
  'messages-square': MessagesSquare,
  'folder-kanban': FolderKanban,
  'calendar-range': CalendarRange,
  target: Target,
  'building-2': Building2,
  archive: Archive,
  trophy: Trophy,
  // general purpose
  monitor: Monitor,
  laptop: Monitor,
  wrench: Wrench,
  'shopping-cart': ShoppingCart,
  truck: Truck,
  'pen-tool': PenTool,
  'hard-hat': HardHat,
  building: Building2,
  'shield-check': ShieldCheck,
  shield: ShieldCheck,
  lightbulb: Lightbulb,
  wallet: Wallet,
  headset: Headset,
  folder: Folder,
  workflow: Workflow,
  'monitor-cog': MonitorCog,
  package: Package,
  'drafting-compass': DraftingCompass,
  handshake: Handshake,
  'circle-help': CircleHelp,
};

export const CATEGORY_ICON_NAMES = Object.keys(ICONS).filter((k) => !['laptop', 'building', 'shield'].includes(k));

export function hasCategoryIcon(name: string | undefined): boolean {
  return Boolean(name && Object.prototype.hasOwnProperty.call(ICONS, name));
}

export function CategoryIcon({ name, className }: { name: string | undefined; className?: string }) {
  const Icon = (name && Object.prototype.hasOwnProperty.call(ICONS, name) ? ICONS[name] : undefined) ?? Folder;
  return <Icon className={className ?? 'h-5 w-5'} aria-hidden="true" />;
}
