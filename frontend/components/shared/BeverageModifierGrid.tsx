"use client"

import { Flame, Snowflake } from "lucide-react"
import { formatCurrency } from "@/lib/format"
import type { ItemModifier } from "@/types/api"

export function isBeverageCategory(name: string): boolean {
  return /beverage|drink|coffee|tea|bar|juice|shake|smoothie|cocktail|mocktail/i.test(name)
}

function SizeCircle({ size }: { size: "S" | "M" | "L" }) {
  const r = size === "S" ? 5 : size === "M" ? 7 : 9
  return (
    <svg width={20} height={20} viewBox="0 0 20 20" fill="none" aria-hidden>
      <circle cx={10} cy={10} r={r} stroke="currentColor" strokeWidth={1.5} />
    </svg>
  )
}

function ModifierIcon({ group, name }: { group: string; name: string }) {
  const g = group.toLowerCase()
  const n = name.toLowerCase()

  if (g === "size") {
    if (n.startsWith("s")) return <SizeCircle size="S" />
    if (n.startsWith("m")) return <SizeCircle size="M" />
    if (n.startsWith("l")) return <SizeCircle size="L" />
    return null
  }

  if (g === "temperature") {
    if (/hot|warm/.test(n)) return <Flame size={16} aria-hidden />
    if (/ice|iced|cold/.test(n)) return <Snowflake size={16} aria-hidden />
    return null
  }

  if (g === "milk") {
    return (
      <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1, color: "var(--ink-2)" }}>
        {name.charAt(0).toUpperCase()}
      </span>
    )
  }

  return null
}

function BeverageCell({
  modifier,
  isSelected,
  onToggle,
}: {
  modifier: ItemModifier
  isSelected: boolean
  onToggle: (id: number) => void
}) {
  return (
    <button
      onClick={() => onToggle(modifier.id)}
      className="press"
      style={{
        height: 68,
        borderRadius: 10,
        border: isSelected ? "1.5px solid var(--accent)" : "1px solid var(--line-2)",
        background: isSelected ? "var(--accent-soft)" : "var(--bg-elev-2)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: 3,
        padding: "8px 4px",
        transition: "background var(--dur-fast) var(--ease), border-color var(--dur-fast) var(--ease)",
        color: isSelected ? "var(--accent)" : "var(--ink-2)",
        overflow: "hidden",
      }}
    >
      <ModifierIcon group={modifier.modifier_group ?? ""} name={modifier.name} />
      <span style={{
        fontSize: 12,
        color: "var(--ink-1)",
        lineHeight: 1.2,
        textAlign: "center",
        maxWidth: "100%",
        overflow: "hidden",
        textOverflow: "ellipsis",
        whiteSpace: "nowrap",
        padding: "0 4px",
      }}>
        {modifier.name}
      </span>
      {modifier.price_delta !== 0 && (
        <span style={{ fontSize: 10, color: "var(--ink-3)" }}>
          {modifier.price_delta > 0 ? "+" : ""}{formatCurrency(modifier.price_delta)}
        </span>
      )}
    </button>
  )
}

interface BeverageModifierGroupProps {
  groupName: string
  modifiers: ItemModifier[]
  selected: number[]
  onToggle: (id: number) => void
}

export function BeverageModifierGroup({ groupName, modifiers, selected, onToggle }: BeverageModifierGroupProps) {
  const cols = modifiers.length <= 2 ? "repeat(2, 1fr)" : "repeat(3, 1fr)"
  const hasRequired = modifiers.some((m) => m.is_required)
  const singleSelect = modifiers.some((m) => m.single_select)

  return (
    <div>
      <span className="eyebrow" style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8, textTransform: "capitalize" }}>
        {groupName}
        {hasRequired && (
          <span style={{ color: "var(--alert, #e74c3c)", marginLeft: -4 }}>*</span>
        )}
        {singleSelect && (
          <span style={{ fontSize: 9.5, fontWeight: 700, letterSpacing: "0.08em", textTransform: "uppercase", color: "var(--accent)", background: "var(--accent-soft)", padding: "2px 7px", borderRadius: "var(--rad-pill)" }}>
            Pick one
          </span>
        )}
      </span>
      <div style={{ display: "grid", gridTemplateColumns: cols, gap: 8 }}>
        {modifiers.map((mod) => (
          <BeverageCell
            key={mod.id}
            modifier={mod}
            isSelected={selected.includes(mod.id)}
            onToggle={onToggle}
          />
        ))}
      </div>
    </div>
  )
}
