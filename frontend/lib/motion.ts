export const spring = { type: "spring" as const, damping: 30, stiffness: 300 }
export const springGentle = { type: "spring" as const, damping: 35, stiffness: 200 }
export const springSnappy = { type: "spring" as const, damping: 28, stiffness: 400 }

export const duration = { fast: 0.14, normal: 0.26, slow: 0.42 }

export const easing = {
  standard:   [0.22, 0.61, 0.36, 1]      as [number, number, number, number],
  decelerate: [0.16, 1,    0.3,  1]      as [number, number, number, number],
  accelerate: [0.55, 0,    0.85, 0]      as [number, number, number, number],
  back:       [0.34, 1.56, 0.64, 1]      as [number, number, number, number],
}

export const prefersReduced =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false
