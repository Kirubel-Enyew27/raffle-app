import test from 'node:test'
import assert from 'node:assert/strict'

// Mock Web Audio API and DOM environment before importing soundManager
class MockAudioParam {
  value: number = 1
  setValueAtTimeCalls: Array<{ value: number; time: number }> = []
  linearRampCalls: Array<{ value: number; time: number }> = []
  exponentialRampCalls: Array<{ value: number; time: number }> = []
  cancelScheduledCalls: Array<{ time: number }> = []

  setValueAtTime(value: number, time: number) {
    this.value = value
    this.setValueAtTimeCalls.push({ value, time })
  }
  linearRampToValueAtTime(value: number, time: number) {
    this.value = value
    this.linearRampCalls.push({ value, time })
  }
  exponentialRampToValueAtTime(value: number, time: number) {
    this.value = value
    this.exponentialRampCalls.push({ value, time })
  }
  cancelScheduledValues(time: number) {
    this.cancelScheduledCalls.push({ time })
  }
}

class MockGainNode {
  gain = new MockAudioParam()
  connectedTo: any = null
  connect(dest: any) {
    this.connectedTo = dest
  }
}

class MockOscillatorNode {
  type: string = 'sine'
  frequency = new MockAudioParam()
  connectedTo: any = null
  started: boolean = false
  stopped: boolean = false
  startTime: number = 0
  stopTime: number = 0

  connect(dest: any) {
    this.connectedTo = dest
  }
  start(time = 0) {
    this.started = true
    this.startTime = time
  }
  stop(time = 0) {
    this.stopped = true
    this.stopTime = time
  }
}

let createdOscillators: MockOscillatorNode[] = []
let createdGains: MockGainNode[] = []

class MockAudioContext {
  state: 'suspended' | 'running' | 'closed' = 'suspended'
  currentTime: number = 0
  destination = {}

  createGain() {
    const g = new MockGainNode()
    createdGains.push(g)
    return g
  }

  createOscillator() {
    const osc = new MockOscillatorNode()
    createdOscillators.push(osc)
    return osc
  }

  async resume() {
    this.state = 'running'
  }

  async close() {
    this.state = 'closed'
  }
}

// Set up global mocks
const storage = new Map<string, string>()
;(globalThis as any).localStorage = {
  getItem: (key: string) => storage.get(key) ?? null,
  setItem: (key: string, value: string) => storage.set(key, String(value)),
  removeItem: (key: string) => storage.delete(key),
  clear: () => storage.clear(),
}

// Track window listeners so cleanup can be asserted (the sound manager must
// remove exactly the handlers it registered).
const windowListeners = new Map<string, Set<unknown>>()
;(globalThis as any).window = globalThis
;(globalThis as any).window.AudioContext = MockAudioContext
;(globalThis as any).window.addEventListener = (type: string, handler: unknown) => {
  if (!windowListeners.has(type)) windowListeners.set(type, new Set())
  windowListeners.get(type)!.add(handler)
}
;(globalThis as any).window.removeEventListener = (type: string, handler: unknown) => {
  windowListeners.get(type)?.delete(handler)
}

// Now import SoundManager
const { soundManager } = await import('./soundManager.ts')

test('SoundManager - Default state and initialization', () => {
  assert.equal(typeof soundManager.playTurnAlert, 'function')
  assert.equal(typeof soundManager.playMove, 'function')
  assert.equal(typeof soundManager.playTimerTick, 'function')
  assert.equal(typeof soundManager.playVictory, 'function')
  assert.equal(typeof soundManager.playDefeat, 'function')
  assert.equal(typeof soundManager.playDraw, 'function')
  assert.equal(typeof soundManager.playMatchStart, 'function')
})

test('SoundManager - Volume control and bounds clamping', () => {
  soundManager.setVolume(0.8)
  assert.equal(soundManager.getVolume(), 0.8)

  // Clamp upper bound
  soundManager.setVolume(1.5)
  assert.equal(soundManager.getVolume(), 1.0)

  // Clamp lower bound
  soundManager.setVolume(-0.5)
  assert.equal(soundManager.getVolume(), 0.0)

  soundManager.setVolume(0.3)
})

test('SoundManager - Mute toggle and persistence', () => {
  soundManager.setMuted(true)
  assert.equal(soundManager.isMuted(), true)

  createdOscillators = []
  soundManager.playTurnAlert()
  soundManager.playMove()
  soundManager.playVictory()
  // When muted, no sound oscillators should be started
  assert.equal(createdOscillators.length, 0)

  // Unmute
  soundManager.setMuted(false)
  assert.equal(soundManager.isMuted(), false)

  soundManager.playMove()
  assert.ok(createdOscillators.length > 0)
})

test('SoundManager - Synthesis of Turn Alert', () => {
  soundManager.setMuted(false)
  createdOscillators = []

  soundManager.playTurnAlert()
  // Turn alert uses two ascending tones (E5 ~659Hz, A5 ~880Hz)
  assert.equal(createdOscillators.length, 2)
  assert.equal(createdOscillators[0].started, true)
  assert.equal(createdOscillators[1].started, true)

  const f1 = createdOscillators[0].frequency.setValueAtTimeCalls[0].value
  const f2 = createdOscillators[1].frequency.setValueAtTimeCalls[0].value
  assert.ok(Math.abs(f1 - 659.25) < 1, `expected ~659Hz, got ${f1}`)
  assert.ok(Math.abs(f2 - 880.0) < 1, `expected ~880Hz, got ${f2}`)
})

test('SoundManager - Synthesis of Move Placement', () => {
  soundManager.setMuted(false)
  createdOscillators = []

  soundManager.playMove()
  assert.equal(createdOscillators.length, 1)
  const osc = createdOscillators[0]
  assert.equal(osc.type, 'triangle')
  assert.equal(osc.started, true)
  assert.equal(osc.stopped, true)
  // Pitch drop envelope (380 -> 90)
  assert.equal(osc.frequency.setValueAtTimeCalls[0].value, 380)
  assert.equal(osc.frequency.exponentialRampCalls[0].value, 90)
})

test('SoundManager - Synthesis of Capture', () => {
  soundManager.setMuted(false)
  createdOscillators = []

  soundManager.playCapture()
  assert.equal(createdOscillators.length, 1)
  const osc = createdOscillators[0]
  assert.equal(osc.type, 'sawtooth')
  assert.equal(osc.frequency.setValueAtTimeCalls[0].value, 260)
  assert.equal(osc.frequency.exponentialRampCalls[0].value, 85)
  assert.equal(osc.started, true)
  assert.equal(osc.stopped, true)
})

test('SoundManager - Synthesis of Promotion', () => {
  soundManager.setMuted(false)
  createdOscillators = []

  soundManager.playPromotion()
  assert.equal(createdOscillators.length, 1)
  const osc = createdOscillators[0]
  assert.equal(osc.type, 'sine')
  assert.equal(osc.frequency.setValueAtTimeCalls[0].value, 520)
  assert.equal(osc.frequency.exponentialRampCalls[0].value, 1040)
  assert.equal(osc.started, true)
  assert.equal(osc.stopped, true)
})

test('SoundManager - Synthesis of Timer Tick', () => {
  soundManager.setMuted(false)
  createdOscillators = []

  soundManager.playTimerTick(4)
  assert.equal(createdOscillators.length, 1)
  assert.equal(createdOscillators[0].type, 'sine')
  assert.equal(createdOscillators[0].frequency.setValueAtTimeCalls[0].value, 920)

  createdOscillators = []
  // Final 2 seconds use slightly higher pitch (1100 Hz)
  soundManager.playTimerTick(2)
  assert.equal(createdOscillators.length, 1)
  assert.equal(createdOscillators[0].frequency.setValueAtTimeCalls[0].value, 1100)
})

test('SoundManager - Synthesis of Victory and Defeat', () => {
  soundManager.setMuted(false)

  // Victory (4 major arpeggio notes)
  createdOscillators = []
  soundManager.playVictory()
  assert.equal(createdOscillators.length, 4)
  assert.ok(createdOscillators.every((o) => o.started && o.stopped))

  // Defeat (2 descending notes)
  createdOscillators = []
  soundManager.playDefeat()
  assert.equal(createdOscillators.length, 2)
  assert.ok(createdOscillators.every((o) => o.started && o.stopped))

  // Draw (2 harmonic notes)
  createdOscillators = []
  soundManager.playDraw()
  assert.equal(createdOscillators.length, 2)
})

test('SoundManager - Listener cleanup removes the registered handlers', () => {
  // The singleton registers unlock listeners in its constructor.
  assert.ok((windowListeners.get('pointerdown')?.size ?? 0) >= 1)
  assert.ok((windowListeners.get('keydown')?.size ?? 0) >= 1)

  soundManager.destroy()

  assert.equal(windowListeners.get('pointerdown')?.size ?? 0, 0)
  assert.equal(windowListeners.get('keydown')?.size ?? 0, 0)
})

test('SoundManager - Resilience to AudioContext failures', () => {
  // If getContext returns null or fails, play methods should not throw
  const brokenManager = new (soundManager.constructor as any)()
  brokenManager.getContext = () => null

  assert.doesNotThrow(() => brokenManager.playTurnAlert())
  assert.doesNotThrow(() => brokenManager.playMove())
  assert.doesNotThrow(() => brokenManager.playTimerTick(3))
  assert.doesNotThrow(() => brokenManager.playVictory())
  assert.doesNotThrow(() => brokenManager.playDefeat())
  assert.doesNotThrow(() => brokenManager.playDraw())
  assert.doesNotThrow(() => brokenManager.playMatchStart())
})
