import { ChefHat, Flame, Leaf, Sparkles } from "lucide-react"
import type { DietaryFlag, ItemBadge } from "@/types/api"

const pillStyle: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 4,
  borderRadius: "var(--rad-pill)",
  padding: "3px 8px",
  fontSize: 11,
  fontWeight: 600,
  letterSpacing: "0.05em",
  textTransform: "uppercase",
  lineHeight: 1,
}

// Inline SVG icons for dietary flags
function VegCircle({ color }: { color: string }) {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <circle cx="5" cy="5" r="4.5" stroke={color} strokeWidth="1" fill={color} />
    </svg>
  )
}

function NonVegTriangle({ color }: { color: string }) {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <polygon points="5,1 9.5,9 0.5,9" fill={color} />
    </svg>
  )
}

function VeganLeaf({ color }: { color: string }) {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <path d="M5 1 C8 1 9 4 9 6 C9 8 7 9 5 9 C3 9 1 8 1 6 C1 4 2 1 5 1 Z" fill={color} />
      <line x1="5" y1="9" x2="5" y2="4" stroke="white" strokeWidth="0.8" />
    </svg>
  )
}

function JainCircle({ color }: { color: string }) {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <circle cx="5" cy="5" r="4.5" stroke={color} strokeWidth="1" fill="none" />
      <text x="5" y="7.5" textAnchor="middle" fontSize="6" fontWeight="700" fill={color} fontFamily="sans-serif">J</text>
    </svg>
  )
}

function EggShape({ color }: { color: string }) {
  return (
    <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <ellipse cx="5" cy="5.5" rx="3.2" ry="4" stroke={color} strokeWidth="1" fill="none" />
    </svg>
  )
}

const dietaryConfig: Record<DietaryFlag, { bg: string; text: string; label: string; icon: (c: string) => React.ReactNode }> = {
  vegetarian: {
    bg: "var(--ok-soft)",
    text: "var(--ok)",
    label: "Veg",
    icon: (c) => <VegCircle color={c} />,
  },
  vegan: {
    bg: "var(--ok-soft)",
    text: "var(--ok)",
    label: "Vegan",
    icon: (c) => <VeganLeaf color={c} />,
  },
  jain: {
    bg: "rgba(201,135,58,0.15)",
    text: "var(--warn)",
    label: "Jain",
    icon: (c) => <JainCircle color={c} />,
  },
  egg: {
    bg: "var(--warn-soft)",
    text: "var(--warn)",
    label: "Egg",
    icon: (c) => <EggShape color={c} />,
  },
  "non-veg": {
    bg: "var(--alert-soft)",
    text: "var(--alert)",
    label: "Non-Veg",
    icon: (c) => <NonVegTriangle color={c} />,
  },
}

export function DietaryTag({ flag }: { flag: DietaryFlag }) {
  const cfg = dietaryConfig[flag]
  return (
    <span style={{ ...pillStyle, background: cfg.bg, color: cfg.text }}>
      {cfg.icon(cfg.text)}
      {cfg.label}
    </span>
  )
}

const badgeConfig: Record<ItemBadge, { bg: string; text: string; label: string; icon: React.ReactNode }> = {
  "chef-special": {
    bg: "var(--accent-soft)",
    text: "var(--accent)",
    label: "Chef Special",
    icon: <ChefHat size={10} />,
  },
  bestseller: {
    bg: "var(--warn-soft)",
    text: "var(--warn)",
    label: "Bestseller",
    icon: <Flame size={10} />,
  },
  seasonal: {
    bg: "var(--info-soft)",
    text: "var(--info)",
    label: "Seasonal",
    icon: <Leaf size={10} />,
  },
  new: {
    bg: "var(--info-soft)",
    text: "var(--info)",
    label: "New",
    icon: <Sparkles size={10} />,
  },
}

export function BadgeTag({ badge }: { badge: ItemBadge }) {
  const cfg = badgeConfig[badge]
  return (
    <span style={{ ...pillStyle, background: cfg.bg, color: cfg.text }}>
      {cfg.icon}
      {cfg.label}
    </span>
  )
}

function ChiliIcon({ size = 12, color = "var(--alert)" }: { size?: number; color?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 12 12" fill="none" aria-hidden="true">
      {/* stem */}
      <path d="M6 1 Q7 2 6.5 3" stroke="var(--ok)" strokeWidth="1" strokeLinecap="round" fill="none" />
      {/* body */}
      <path d="M6.5 3 Q9 3.5 9 6.5 Q9 10 6 10.5 Q3 10 3 6.5 Q3 3.5 6.5 3 Z" fill={color} />
    </svg>
  )
}

export function SpiceIndicator({ level }: { level: number }) {
  if (!level || level === 0) return null
  return (
    <span style={{ display: "inline-flex", gap: 2, alignItems: "center" }}>
      {Array.from({ length: level }).map((_, i) => (
        <ChiliIcon key={i} size={12} color="var(--alert)" />
      ))}
    </span>
  )
}
