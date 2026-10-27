import { useEffect, useRef } from 'react'

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  color: string
  size: number
  rotation: number
  vRot: number
  alpha: number
}

const COLORS = [
  '#10b981', // emerald
  '#06b6d4', // cyan
  '#3b82f6', // blue
  '#f59e0b', // amber
  '#ec4899', // pink
  '#8b5cf6', // purple
]

export function VictoryConfetti({ durationMs = 3500 }: { durationMs?: number }) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let animId: number
    const dpr = window.devicePixelRatio || 1
    const width = (canvas.width = canvas.offsetWidth * dpr)
    const height = (canvas.height = canvas.offsetHeight * dpr)
    ctx.scale(dpr, dpr)

    const count = 55
    const particles: Particle[] = Array.from({ length: count }, () => ({
      x: (canvas.offsetWidth / 2) + (Math.random() - 0.5) * 160,
      y: canvas.offsetHeight * 0.25 + (Math.random() - 0.5) * 60,
      vx: (Math.random() - 0.5) * 7,
      vy: -Math.random() * 6 - 2,
      color: COLORS[Math.floor(Math.random() * COLORS.length)],
      size: Math.random() * 5 + 4,
      rotation: Math.random() * 360,
      vRot: (Math.random() - 0.5) * 8,
      alpha: 1,
    }))

    const start = performance.now()

    const render = (now: number) => {
      const elapsed = now - start
      if (elapsed > durationMs) {
        ctx.clearRect(0, 0, width, height)
        return
      }

      ctx.clearRect(0, 0, width, height)
      const fadeProgress = elapsed / durationMs
      const globalAlpha = fadeProgress > 0.7 ? 1 - (fadeProgress - 0.7) / 0.3 : 1

      for (const p of particles) {
        p.x += p.vx
        p.y += p.vy
        p.vy += 0.16 // gravity
        p.rotation += p.vRot
        p.alpha = Math.max(0, globalAlpha)

        ctx.save()
        ctx.globalAlpha = p.alpha
        ctx.fillStyle = p.color
        ctx.translate(p.x, p.y)
        ctx.rotate((p.rotation * Math.PI) / 180)
        ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.6)
        ctx.restore()
      }

      animId = requestAnimationFrame(render)
    }

    animId = requestAnimationFrame(render)

    return () => {
      cancelAnimationFrame(animId)
    }
  }, [durationMs])

  return (
    <canvas
      ref={canvasRef}
      className="pointer-events-none absolute inset-0 z-30 h-full w-full"
      aria-hidden="true"
    />
  )
}
