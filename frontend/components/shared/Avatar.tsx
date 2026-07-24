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

  // Hue comes from the name; saturation/lightness/ink read from theme tokens with
  // sensible fallbacks so a surface can retune them. The blob stays mid-dark and the
  // ink stays light, so initials keep contrast on both light and dark surfaces
  // (previously it used --ink-1, which inverted to dark text on light themes).
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
        background: `linear-gradient(135deg, hsl(${hue}, var(--avatar-sat-1, 55%), var(--avatar-l1, 42%)), hsl(${(hue + 30) % 360}, var(--avatar-sat-2, 42%), var(--avatar-l2, 32%)))`,
        boxShadow: "var(--shadow-1)",
        fontSize: size * 0.42,
        fontWeight: 600,
        color: "var(--avatar-ink, #fff)",
        userSelect: "none",
        letterSpacing: 0,
      }}
    >
      {initial}
    </div>
  )
}
