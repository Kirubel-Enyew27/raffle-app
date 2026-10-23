/**
 * Web Audio API Sound Manager for Raffle-App P2P Games
 *
 * Provides lightweight, synthesized, non-blocking sound effects without external audio assets.
 * Respects browser autoplay restrictions by unlocking on first user gesture.
 * Features master gain control, instant muting, and lifecycle management.
 */

export type SoundEffectType =
  | 'turn_alert'
  | 'piece_placement'
  | 'timer_tick'
  | 'victory'
  | 'defeat'
  | 'draw'
  | 'match_start'
  | 'capture'
  | 'promotion'

const STORAGE_KEY = 'raffle_p2p_sound_enabled'
const DEFAULT_VOLUME = 0.3

class SoundManager {
  private ctx: AudioContext | null = null
  private masterGain: GainNode | null = null
  private muted: boolean = false
  private volume: number = DEFAULT_VOLUME
  private initialized: boolean = false
  private unlockListenersAttached: boolean = false
  // Must hold the exact handler references so they can be removed again.
  private unlockHandlers: { pointer: () => void; key: () => void } | null = null

  constructor() {
    // Read persisted sound preference (defaults to enabled)
    if (typeof window !== 'undefined') {
      try {
        const stored = localStorage.getItem(STORAGE_KEY)
        this.muted = stored === 'false'
      } catch {
        this.muted = false
      }
      this.attachUnlockListeners()
    }
  }

  /**
   * Lazily initialize or retrieve the AudioContext.
   */
  private getContext(): AudioContext | null {
    if (typeof window === 'undefined') return null

    if (!this.ctx) {
      try {
        const AudioCtx = window.AudioContext || (window as any).webkitAudioContext
        if (!AudioCtx) return null

        this.ctx = new AudioCtx()
        this.masterGain = this.ctx.createGain()
        this.masterGain.gain.setValueAtTime(this.muted ? 0 : this.volume, this.ctx.currentTime)
        this.masterGain.connect(this.ctx.destination)
        this.initialized = true
      } catch (err) {
        console.warn('[SoundManager] Web Audio API failed to initialize:', err)
        return null
      }
    }

    if (this.ctx && this.ctx.state === 'suspended') {
      this.ctx.resume().catch(() => {})
    }

    return this.ctx
  }

  /**
   * Listen for user gestures to unlock AudioContext according to browser autoplay policy.
   */
  private attachUnlockListeners(): void {
    if (this.unlockListenersAttached || typeof window === 'undefined') return

    const unlock = () => {
      if (this.ctx && this.ctx.state === 'suspended') {
        this.ctx.resume().then(() => {
          this.removeUnlockListeners()
        }).catch(() => {})
      } else if (this.ctx && this.ctx.state === 'running') {
        this.removeUnlockListeners()
      }
    }

    this.unlockHandlers = { pointer: unlock, key: unlock }
    window.addEventListener('pointerdown', unlock, { passive: true })
    window.addEventListener('keydown', unlock, { passive: true })
    this.unlockListenersAttached = true
  }

  private removeUnlockListeners(): void {
    if (typeof window === 'undefined') return
    if (this.unlockHandlers) {
      window.removeEventListener('pointerdown', this.unlockHandlers.pointer)
      window.removeEventListener('keydown', this.unlockHandlers.key)
      this.unlockHandlers = null
    }
    this.unlockListenersAttached = false
  }

  /**
   * Set mute status and persist preference.
   */
  public setMuted(mute: boolean): void {
    this.muted = mute
    try {
      localStorage.setItem(STORAGE_KEY, String(!mute))
    } catch {
      // Ignore storage errors in private mode
    }

    if (this.masterGain && this.ctx) {
      const now = this.ctx.currentTime
      this.masterGain.gain.cancelScheduledValues(now)
      this.masterGain.gain.setValueAtTime(mute ? 0 : this.volume, now)
    }
  }

  public isMuted(): boolean {
    return this.muted
  }

  public isInitialized(): boolean {
    return this.initialized
  }

  public toggleMute(): boolean {
    this.setMuted(!this.muted)
    return this.muted
  }

  public setVolume(vol: number): void {
    this.volume = Math.max(0, Math.min(1, vol))
    if (this.masterGain && this.ctx && !this.muted) {
      const now = this.ctx.currentTime
      this.masterGain.gain.cancelScheduledValues(now)
      this.masterGain.gain.setValueAtTime(this.volume, now)
    }
  }

  public getVolume(): number {
    return this.volume
  }

  // ---------------------------------------------------------------------------
  // Synthesized Sound Generators
  // ---------------------------------------------------------------------------

  /**
   * Turn Alert: Bright, friendly ascending two-tone chime.
   * Plays when it becomes the local user's turn.
   */
  public playTurnAlert(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime

      // Tone 1: E5 (659.25 Hz)
      const osc1 = ctx.createOscillator()
      const gain1 = ctx.createGain()
      osc1.type = 'sine'
      osc1.frequency.setValueAtTime(659.25, now)

      gain1.gain.setValueAtTime(0, now)
      gain1.gain.linearRampToValueAtTime(0.25, now + 0.02)
      gain1.gain.exponentialRampToValueAtTime(0.001, now + 0.12)

      osc1.connect(gain1)
      gain1.connect(this.masterGain)
      osc1.start(now)
      osc1.stop(now + 0.12)

      // Tone 2: A5 (880 Hz) - slightly delayed
      const osc2 = ctx.createOscillator()
      const gain2 = ctx.createGain()
      osc2.type = 'sine'
      osc2.frequency.setValueAtTime(880, now + 0.08)

      gain2.gain.setValueAtTime(0, now + 0.08)
      gain2.gain.linearRampToValueAtTime(0.3, now + 0.1)
      gain2.gain.exponentialRampToValueAtTime(0.001, now + 0.28)

      osc2.connect(gain2)
      gain2.connect(this.masterGain)
      osc2.start(now + 0.08)
      osc2.stop(now + 0.28)
    } catch {
      // Audio playback failure must never interrupt gameplay
    }
  }

  /**
   * Piece Placement / Move: Crisp, snappy wooden/plastic pop tap.
   * Plays when a valid move is accepted into game state.
   */
  public playMove(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      // Quick pitch drop creates a realistic tactile click/pop
      osc.type = 'triangle'
      osc.frequency.setValueAtTime(380, now)
      osc.frequency.exponentialRampToValueAtTime(90, now + 0.06)

      gain.gain.setValueAtTime(0, now)
      gain.gain.linearRampToValueAtTime(0.28, now + 0.005)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.06)

      osc.connect(gain)
      gain.connect(this.masterGain)
      osc.start(now)
      osc.stop(now + 0.06)
    } catch {
      // Safe no-op
    }
  }

  /**
   * Capture: A heavier, lower double-thud for a jumped piece. Deliberately
   * distinct from the ordinary placement pop so Dama players can hear that a
   * mandatory capture happened.
   */
  public playCapture(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      osc.type = 'sawtooth'
      osc.frequency.setValueAtTime(260, now)
      osc.frequency.exponentialRampToValueAtTime(85, now + 0.14)

      gain.gain.setValueAtTime(0, now)
      gain.gain.linearRampToValueAtTime(0.32, now + 0.006)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.14)

      osc.connect(gain)
      gain.connect(this.masterGain)
      osc.start(now)
      osc.stop(now + 0.14)
    } catch {
      // Safe no-op
    }
  }

  /**
   * Promotion: A bright rising chime when a man reaches the back rank and is
   * crowned. Distinct from victory so it reads as "this piece changed".
   */
  public playPromotion(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      osc.type = 'sine'
      osc.frequency.setValueAtTime(520, now)
      osc.frequency.exponentialRampToValueAtTime(1040, now + 0.22)

      gain.gain.setValueAtTime(0, now)
      gain.gain.linearRampToValueAtTime(0.24, now + 0.02)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.24)

      osc.connect(gain)
      gain.connect(this.masterGain)
      osc.start(now)
      osc.stop(now + 0.24)
    } catch {
      // Safe no-op
    }
  }

  /**
   * Timer Tick: Soft, subtle wooden clock tick.
   * Configured for the final seconds of countdown (5s, 4s, 3s, 2s, 1s).
   */
  public playTimerTick(secondsLeft?: number): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      // Slightly higher pitch on urgent final 2 seconds
      const freq = secondsLeft !== undefined && secondsLeft <= 2 ? 1100 : 920

      osc.type = 'sine'
      osc.frequency.setValueAtTime(freq, now)

      gain.gain.setValueAtTime(0, now)
      gain.gain.linearRampToValueAtTime(0.12, now + 0.003)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.035)

      osc.connect(gain)
      gain.connect(this.masterGain)
      osc.start(now)
      osc.stop(now + 0.035)
    } catch {
      // Safe no-op
    }
  }

  /**
   * Victory: Uplifting major arpeggio celebrating match triumph.
   * Plays only once when local player wins a finalized match.
   */
  public playVictory(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      // C Major arpeggio notes: C5 (523.25), E5 (659.25), G5 (783.99), C6 (1046.50)
      const notes = [
        { freq: 523.25, time: 0.00, dur: 0.16 },
        { freq: 659.25, time: 0.10, dur: 0.16 },
        { freq: 783.99, time: 0.20, dur: 0.18 },
        { freq: 1046.50, time: 0.30, dur: 0.45 },
      ]

      notes.forEach(({ freq, time, dur }) => {
        const osc = ctx.createOscillator()
        const gain = ctx.createGain()
        osc.type = 'triangle'
        osc.frequency.setValueAtTime(freq, now + time)

        gain.gain.setValueAtTime(0, now + time)
        gain.gain.linearRampToValueAtTime(0.22, now + time + 0.02)
        gain.gain.exponentialRampToValueAtTime(0.001, now + time + dur)

        osc.connect(gain)
        gain.connect(this.masterGain!)
        osc.start(now + time)
        osc.stop(now + time + dur)
      })
    } catch {
      // Safe no-op
    }
  }

  /**
   * Defeat: Gentle, respectful low-key descending motif.
   * Plays only once when local player loses a finalized match.
   */
  public playDefeat(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      // Descending minor interval: E4 (329.63) -> C4 (261.63)
      const notes = [
        { freq: 329.63, time: 0.00, dur: 0.22 },
        { freq: 261.63, time: 0.18, dur: 0.35 },
      ]

      notes.forEach(({ freq, time, dur }) => {
        const osc = ctx.createOscillator()
        const gain = ctx.createGain()
        osc.type = 'sine'
        osc.frequency.setValueAtTime(freq, now + time)

        gain.gain.setValueAtTime(0, now + time)
        gain.gain.linearRampToValueAtTime(0.18, now + time + 0.03)
        gain.gain.exponentialRampToValueAtTime(0.001, now + time + dur)

        osc.connect(gain)
        gain.connect(this.masterGain!)
        osc.start(now + time)
        osc.stop(now + time + dur)
      })
    } catch {
      // Safe no-op
    }
  }

  /**
   * Draw / Tie: Harmonious, balanced two-chord resolution.
   */
  public playDraw(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const frequencies = [440.0, 554.37] // A4 & C#5 dual tone

      frequencies.forEach((freq) => {
        const osc = ctx.createOscillator()
        const gain = ctx.createGain()
        osc.type = 'sine'
        osc.frequency.setValueAtTime(freq, now)

        gain.gain.setValueAtTime(0, now)
        gain.gain.linearRampToValueAtTime(0.16, now + 0.02)
        gain.gain.exponentialRampToValueAtTime(0.001, now + 0.32)

        osc.connect(gain)
        gain.connect(this.masterGain!)
        osc.start(now)
        osc.stop(now + 0.32)
      })
    } catch {
      // Safe no-op
    }
  }

  /**
   * Match Start: Energetic ready chime when entering the duel.
   */
  public playMatchStart(): void {
    if (this.muted) return
    const ctx = this.getContext()
    if (!ctx || !this.masterGain) return

    try {
      const now = ctx.currentTime
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()

      osc.type = 'sine'
      osc.frequency.setValueAtTime(440, now)
      osc.frequency.exponentialRampToValueAtTime(659.25, now + 0.15)

      gain.gain.setValueAtTime(0, now)
      gain.gain.linearRampToValueAtTime(0.2, now + 0.02)
      gain.gain.exponentialRampToValueAtTime(0.001, now + 0.2)

      osc.connect(gain)
      gain.connect(this.masterGain)
      osc.start(now)
      osc.stop(now + 0.2)
    } catch {
      // Safe no-op
    }
  }

  /**
   * Cleanup resources if needed on app destroy.
   */
  public destroy(): void {
    this.removeUnlockListeners()
    if (this.ctx && this.ctx.state !== 'closed') {
      this.ctx.close().catch(() => {})
      this.ctx = null
      this.masterGain = null
    }
  }
}

// Singleton instance for global reuse across all games and components
export const soundManager = new SoundManager()
