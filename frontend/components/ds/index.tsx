import * as React from "react"
import { cn } from "@/lib/utils"

/* Design-system primitives for the ops / platform surfaces.
 *
 * Surface-agnostic: they read the governed theme tokens (var(--bg-*), var(--ink-*),
 * var(--accent*), var(--line-*), var(--shadow-*), var(--rad-*), var(--font-*)) which
 * resolve per [data-surface="ops"|"platform"] (and would resolve per [data-theme] on
 * guest). Implemented with inline token styles + Tailwind layout utilities to match
 * this codebase's conventions (the Harmony reference's bg-surface/text-ink utilities
 * don't exist here). Mirrors the reference component APIs.
 */

// ───────────────────────── Button ─────────────────────────
type ButtonVariant = "primary" | "brand" | "secondary" | "ghost" | "danger" | "link"
type ButtonSize = "sm" | "md" | "lg" | "xl" | "icon"

const BTN_SIZE: Record<ButtonSize, string> = {
  sm: "h-8 px-3 text-[13px]",
  md: "h-10 px-4 text-[14px]",
  lg: "h-12 px-5 text-[15px]",
  xl: "h-14 px-6 text-[16px]",
  icon: "h-9 w-9",
}
const BTN_RAD: Record<ButtonSize, string> = {
  sm: "var(--rad-sm)", md: "var(--rad-sm)", lg: "var(--rad-md)", xl: "var(--rad-lg)", icon: "var(--rad-sm)",
}
function btnStyle(variant: ButtonVariant): React.CSSProperties {
  switch (variant) {
    case "brand":     return { background: "var(--accent)", color: "var(--accent-ink)" }
    case "secondary": return { background: "var(--bg-elev-1)", color: "var(--ink-1)", border: "1px solid var(--line-2)" }
    case "ghost":     return { background: "transparent", color: "var(--ink-1)" }
    case "danger":    return { background: "var(--alert)", color: "var(--accent-ink)" }
    case "link":      return { background: "transparent", color: "var(--ink-1)", padding: 0, height: "auto", textDecorationLine: "underline", textUnderlineOffset: 4 }
    case "primary":
    default:          return { background: "var(--ink-1)", color: "var(--bg-base)" }
  }
}

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "primary", size = "md", style, ...p }, ref) => (
    <button
      ref={ref}
      className={cn(
        "press inline-flex items-center justify-center gap-2 whitespace-nowrap font-medium outline-none transition-[background,color,box-shadow,transform] active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none",
        BTN_SIZE[size],
        className,
      )}
      style={{ borderRadius: BTN_RAD[size], ...btnStyle(variant), ...style }}
      {...p}
    />
  ),
)
Button.displayName = "Button"

// ───────────────────────── Card ─────────────────────────
export function Card({ className, style, ...p }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(className)}
      style={{
        background: "var(--bg-elev-1)",
        border: "1px solid var(--line-2)",
        borderRadius: "var(--rad-lg)",
        boxShadow: "var(--shadow-1)",
        ...style,
      }}
      {...p}
    />
  )
}

// ───────────────────────── Badge ─────────────────────────
type Tone = "neutral" | "brand" | "ok" | "warn" | "danger" | "info" | "outline"
function toneStyle(tone: Tone): React.CSSProperties {
  switch (tone) {
    case "brand":   return { background: "var(--accent-soft)", color: "var(--accent)" }
    case "ok":      return { background: "var(--ok-soft)", color: "var(--ok)" }
    case "warn":    return { background: "var(--warn-soft)", color: "var(--warn)" }
    case "danger":  return { background: "var(--alert-soft)", color: "var(--alert)" }
    case "info":    return { background: "var(--info-soft)", color: "var(--info)" }
    case "outline": return { background: "transparent", color: "var(--ink-2)", border: "1px solid var(--line-2)" }
    case "neutral":
    default:        return { background: "var(--bg-elev-2)", color: "var(--ink-2)" }
  }
}
export function Badge({ className, tone = "neutral", style, ...p }:
  React.HTMLAttributes<HTMLSpanElement> & { tone?: Tone }) {
  return (
    <span
      className={cn("inline-flex items-center gap-1.5 px-2 h-[22px] rounded-full text-[11px] font-medium tracking-wide uppercase", className)}
      style={{ ...toneStyle(tone), ...style }}
      {...p}
    />
  )
}

// ───────────────────────── StatusDot ─────────────────────────
const DOT_COLOR: Record<string, string> = {
  ok: "var(--ok)", warn: "var(--warn)", danger: "var(--alert)", info: "var(--info)", neutral: "var(--ink-3)",
}
export function StatusDot({ tone = "ok", pulse }: { tone?: "ok" | "warn" | "danger" | "info" | "neutral"; pulse?: boolean }) {
  const c = DOT_COLOR[tone] ?? DOT_COLOR.ok
  return (
    <span className="relative inline-flex h-2 w-2">
      {pulse && <span className="absolute inset-0 rounded-full opacity-50 animate-ping" style={{ background: c }} />}
      <span className="relative h-2 w-2 rounded-full" style={{ background: c }} />
    </span>
  )
}

// ───────────────────────── Input ─────────────────────────
export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, style, ...p }, ref) => (
    <input
      ref={ref}
      className={cn("h-10 w-full px-3 text-[14px] outline-none transition-shadow placeholder:text-[color:var(--ink-3)]", className)}
      style={{ borderRadius: "var(--rad-sm)", background: "var(--bg-elev-1)", border: "1px solid var(--line-2)", color: "var(--ink-1)", ...style }}
      {...p}
    />
  ),
)
Input.displayName = "Input"

// ───────────────────────── Field ─────────────────────────
export function Field({ label, hint, children, htmlFor }: { label: string; hint?: string; children: React.ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="block space-y-1.5">
      <span className="block text-[12px] font-medium uppercase tracking-wide" style={{ color: "var(--ink-3)" }}>{label}</span>
      {children}
      {hint && <span className="block text-[12px]" style={{ color: "var(--ink-3)" }}>{hint}</span>}
    </label>
  )
}

// ───────────────────────── Stat ─────────────────────────
export function Stat({ label, value, delta, tone = "neutral" }: { label: string; value: React.ReactNode; delta?: string; tone?: "neutral" | "ok" | "danger" }) {
  const deltaColor = tone === "ok" ? "var(--ok)" : tone === "danger" ? "var(--alert)" : "var(--ink-2)"
  return (
    <div className="space-y-1.5">
      <div className="text-[11px] font-medium uppercase tracking-wider" style={{ color: "var(--ink-3)" }}>{label}</div>
      <div className="text-[28px] font-semibold tracking-tight" style={{ color: "var(--ink-1)", fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums" }}>{value}</div>
      {delta && <div className="text-[12px]" style={{ color: deltaColor, fontFamily: "var(--font-mono)" }}>{delta}</div>}
    </div>
  )
}

// ───────────────────────── Money ─────────────────────────
export function Money({ amount, currency = "INR", className }: { amount: string | number; currency?: string; className?: string }) {
  const sym = currency === "INR" ? "₹" : currency === "USD" ? "$" : currency === "EUR" ? "€" : currency === "GBP" ? "£" : ""
  return (
    <span className={className} style={{ fontFamily: "var(--font-mono)", fontVariantNumeric: "tabular-nums" }}>
      {sym}{typeof amount === "number" ? amount.toFixed(2) : amount}
    </span>
  )
}

// ───────────────────────── Kbd ─────────────────────────
export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd
      className="inline-flex items-center justify-center min-w-[20px] h-[20px] px-1.5 text-[11px]"
      style={{ fontFamily: "var(--font-mono)", borderRadius: 6, background: "var(--bg-sunken)", border: "1px solid var(--line-2)", color: "var(--ink-2)" }}
    >
      {children}
    </kbd>
  )
}

// ───────────────────────── Segmented ─────────────────────────
export function Segmented<T extends string>({ value, onChange, options, className }: {
  value: T
  onChange: (v: T) => void
  options: { value: T; label: React.ReactNode }[]
  className?: string
}) {
  return (
    <div className={cn("inline-flex p-1 gap-1", className)} style={{ background: "var(--bg-sunken)", borderRadius: "var(--rad-sm)" }}>
      {options.map((o) => {
        const active = value === o.value
        return (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            className="press h-8 px-3 text-[13px] font-medium transition-colors"
            style={{
              borderRadius: 6,
              background: active ? "var(--bg-elev-1)" : "transparent",
              color: active ? "var(--ink-1)" : "var(--ink-3)",
              boxShadow: active ? "var(--shadow-1)" : "none",
            }}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}

// ───────────────────────── Toolbar ─────────────────────────
export function Toolbar({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn("sticky top-0 z-30 flex items-center gap-2 h-12 px-4 backdrop-blur", className)}
      style={{ background: "color-mix(in oklch, var(--bg-base) 80%, transparent)", borderBottom: "1px solid var(--line-1)" }}
    >
      {children}
    </div>
  )
}

// ───────────────────────── Avatar ─────────────────────────
export function Avatar({ name, size = 28, tone }: { name: string; size?: number; tone?: string }) {
  const initials = name.trim().split(/\s+/).map((s) => s[0]).slice(0, 2).join("").toUpperCase()
  const hue = (name.charCodeAt(0) * 13) % 360
  const bg = tone ?? `oklch(0.62 0.12 ${hue})`
  return (
    <span
      aria-label={name}
      className="inline-flex items-center justify-center rounded-full font-semibold"
      style={{ width: size, height: size, background: bg, fontSize: size * 0.4, color: "#fff", flexShrink: 0 }}
    >
      {initials}
    </span>
  )
}

// ───────────────────────── EmptyState ─────────────────────────
export function EmptyState({ icon: Icon, title, hint, action }: {
  icon?: React.ComponentType<{ size?: number; className?: string }>
  title: string
  hint?: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-6 gap-3">
      <div className="flex items-center justify-center h-12 w-12 rounded-full" style={{ background: "var(--bg-elev-2)", color: "var(--ink-3)" }}>
        {Icon ? <Icon size={20} /> : null}
      </div>
      <div className="space-y-1">
        <div className="text-[15px] font-medium" style={{ color: "var(--ink-1)" }}>{title}</div>
        {hint && <div className="text-[13px] max-w-sm" style={{ color: "var(--ink-2)" }}>{hint}</div>}
      </div>
      {action}
    </div>
  )
}
