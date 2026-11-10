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
  type LucideIcon,
} from 'lucide-react';

// Explicit allow-list instead of importing every lucide icon (keeps the bundle small).
const ICONS: Record<string, LucideIcon> = {
  monitor: Monitor,
  laptop: Monitor,
  wrench: Wrench,
  'shopping-cart': ShoppingCart,
  truck: Truck,
  'pen-tool': PenTool,
  'hard-hat': HardHat,
  users: Users,
  'building-2': Building2,
  building: Building2,
  'shield-check': ShieldCheck,
  shield: ShieldCheck,
  lightbulb: Lightbulb,
  wallet: Wallet,
  headset: Headset,
  'file-text': FileText,
  folder: Folder,
  workflow: Workflow,
  'monitor-cog': MonitorCog,
  package: Package,
  'drafting-compass': DraftingCompass,
  handshake: Handshake,
  'circle-help': CircleHelp,
};

export const CATEGORY_ICON_NAMES = Object.keys(ICONS).filter((k) => !['laptop', 'building', 'shield'].includes(k));

export function CategoryIcon({ name, className }: { name: string | undefined; className?: string }) {
  const Icon = (name && ICONS[name]) || Folder;
  return <Icon className={className ?? 'h-5 w-5'} aria-hidden="true" />;
}
