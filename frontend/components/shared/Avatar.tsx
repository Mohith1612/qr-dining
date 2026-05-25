interface AvatarProps {
  name: string
  size?: number
}

function nameToHue(name: string): number {
  return name.split("").reduce((acc, c) => acc + c.charCodeAt(0), 0) % 360
}

export function Avatar({ name, size = 28 }: AvatarProps) {
  const hue = nameToHue(name)
  const initial = name.trim().charAt(0).toUpperCase()

  return (
    <div
      aria-label={name}
      style={{
        width: size,
        height: size,
        borderRadius: "50%",
        flexShrink: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: `linear-gradient(135deg, hsl(${hue}, 55%, 38%), hsl(${(hue + 30) % 360}, 40%, 28%))`,
        boxShadow: "var(--shadow-1)",
        fontSize: size * 0.42,
        fontWeight: 600,
        color: "var(--ink-1)",
        userSelect: "none",
        letterSpacing: 0,
      }}
    >
      {initial}
    </div>
  )
}
