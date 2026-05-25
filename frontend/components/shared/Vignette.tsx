interface VignetteProps {
  hue?: number
  size?: number
  ring?: boolean
  className?: string
}

export function Vignette({ hue = 30, size = 52, ring = false }: VignetteProps) {
  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: "50%",
        flexShrink: 0,
        position: "relative",
        background: `radial-gradient(60% 50% at 35% 35%, hsl(${hue}, 65%, 64%) 0%, hsl(${hue}, 55%, 44%) 45%, hsl(${hue + 8}, 50%, 28%) 100%)`,
        boxShadow: [
          `inset 0 1px 0 hsla(${hue}, 60%, 88%, 0.4)`,
          `inset 0 -8px 16px hsla(${hue + 10}, 40%, 10%, 0.5)`,
          `0 8px 22px hsla(${hue}, 50%, 8%, 0.45)`,
        ].join(", "),
        outline: ring ? "2px solid var(--accent)" : "none",
        outlineOffset: ring ? 2 : 0,
      }}
    />
  )
}
