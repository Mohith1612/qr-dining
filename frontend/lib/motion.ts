export const spring = { type: "spring" as const, damping: 30, stiffness: 300 }
export const springGentle = { type: "spring" as const, damping: 35, stiffness: 200 }
export const springSnappy = { type: "spring" as const, damping: 28, stiffness: 400 }

export const duration = { fast: 0.15, normal: 0.25, slow: 0.4 }

export const easing = {
  standard: [0.2, 0, 0, 1] as [number, number, number, number],
  decelerate: [0, 0, 0.2, 1] as [number, number, number, number],
  accelerate: [0.4, 0, 1, 1] as [number, number, number, number],
}

export const prefersReduced =
  typeof window !== "undefined"
    ? window.matchMedia("(prefers-reduced-motion: reduce)").matches
    : false
