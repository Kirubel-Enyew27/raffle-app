import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  AlertTriangle,
  BadgeCheck,
  CircleDollarSign,
  History,
  Info,
  Loader2,
  ShieldCheck,
  Users,
  WifiOff,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { walletApi } from '@/features/wallet/api'
import {
  crashApi,
  type CrashBet,
  type CrashRound,
  type CrashRoundState,
  type CrashState,
  type CrashVerification,
} from '@/features/crash/api'
import { FlightStage } from '@/features/crash/FlightStage'
import { useCrashStream, type CrashEvent } from '@/features/crash/useCrashStream'

// ---------------------------------------------------------------------------
// Types & helpers
// ---------------------------------------------------------------------------

type State = CrashRoundState | 'unknown'

interface GameState {
  round: CrashRound | null
  state: State
  multiplier: number
  crashMultiplier?: number
  myBet?: CrashBet
  liveBets: CrashBet[]
  recent: CrashRound[]
  previousRound?: CrashRound
  /** Round start time in epoch ms, used only to interpolate the display. */
  startedAtMs?: number
  /**
   * Difference between the server clock and this browser, in ms. Authoritative
   * for countdowns and display interpolation only - never for money.
   */
  offsetMs: number
}

const initialGame: GameState = {
  round: null,
  state: 'unknown',
  multiplier: 1,
  liveBets: [],
  recent: [],
  offsetMs: 0,
}

type GameAction =
  | { type: 'snapshot'; data: CrashState; offsetMs: number }
  | { type: 'event'; event: CrashEvent; offsetMs?: number }
  | { type: 'my_bet'; bet: CrashBet }

/** Small stable id for a player row, used to upsert live-table entries. */
const rowKey = (bet: Pick<CrashBet, 'round_id' | 'user_id'>) => `${bet.round_id}:${bet.user_id}`

const upsertBet = (list: CrashBet[], bet: CrashBet): CrashBet[] => {
  const key = rowKey(bet)
  const without = list.filter((b) => rowKey(b) !== key)
  return [...without, bet]
}

const toMs = (value?: string | null) => (value ? Date.parse(value) : undefined)

function reduceEvent(state: GameState, event: CrashEvent): GameState {
  const payload = event.payload || {}
  const roundId: string | undefined = payload.round_id

  switch (event.type) {
    case 'crash.round_created': {
      // A new round opened: clear the table, keep history, restart the curve.
      const newRound: CrashRound = {
        id: roundId ?? '',
        round_number: payload.round_number ?? 0,
        state: 'WAITING',
        betting_starts_at: payload.betting_starts_at ?? '',
        betting_ends_at: payload.betting_ends_at ?? '',
        crash_multiplier: 0,
        server_seed_hash: payload.server_seed_hash ?? '',
        commitment_published_at: payload.commitment_published_at ?? '',
        nonce: 0,
        total_stake: 0,
        total_payout: 0,
        bet_count: 0,
        cashout_count: 0,
        p2p_mode: (payload as any).p2p_mode ?? true,
        prize_pool: 0,
        service_fee_percent: (payload as any).service_fee_pct ?? 5,
        min_participants: (payload as any).min_participants ?? 2,
        total_fee: 0,
      }
      const previous = state.round && state.round.id !== newRound.id ? state.round : state.previousRound
      return {
        ...state,
        round: newRound,
        state: 'WAITING',
        multiplier: 1,
        crashMultiplier: undefined,
        myBet: undefined,
        liveBets: [],
        startedAtMs: undefined,
        previousRound: previous,
      }
    }

    case 'crash.betting_open':
      return { ...state, state: 'BETTING' }

    case 'crash.round_started':
      return {
        ...state,
        state: 'RUNNING',
        multiplier: 1,
        crashMultiplier: undefined,
        startedAtMs: toMs(payload.started_at),
      }

    case 'crash.tick':
      return { ...state, multiplier: payload.multiplier ?? state.multiplier }

    case 'crash.crashed': {
      // The crash is final: lock the display and show every uncashed bet as lost.
      const crashMultiplier = payload.crash_multiplier ?? state.multiplier
      const round = state.round
        ? { ...state.round, state: 'CRASHED' as CrashRoundState, crash_multiplier: crashMultiplier }
        : state.round
      const markLost = (b: CrashBet) => (b.status === 'active' ? { ...b, status: 'lost' as const } : b)
      return {
        ...state,
        state: 'CRASHED',
        crashMultiplier,
        multiplier: crashMultiplier,
        round,
        liveBets: state.liveBets.map(markLost),
        myBet: state.myBet ? markLost(state.myBet) : undefined,
        recent: round ? [round, ...state.recent.filter((r) => r.id !== round.id)].slice(0, 20) : state.recent,
      }
    }

    case 'crash.settled':
      return {
        ...state,
        state: 'SETTLEMENT',
        round: state.round
          ? {
              ...state.round,
              state: 'SETTLEMENT',
              total_stake: payload.total_stake ?? state.round.total_stake,
              total_payout: payload.total_payout ?? state.round.total_payout,
              bet_count: payload.bet_count ?? state.round.bet_count,
              cashout_count: payload.cashout_count ?? state.round.cashout_count,
              prize_pool: payload.prize_pool ?? state.round.prize_pool,
              total_fee: payload.total_fee ?? state.round.total_fee,
            }
          : state.round,
      }

    case 'crash.p2p_settled': {
      // Update live table bets with their final net_payout from the server.
      const winners: Array<{ user_id: string; net_payout: number; gross_return: number; service_fee: number; gross_profit: number; service_fee_pct: number }> =
        (payload as any).winners ?? []
      const winnerMap = new Map(winners.map((w) => [w.user_id, w]))
      const applySettlement = (bet: CrashBet): CrashBet => {
        const w = winnerMap.get(bet.user_id)
        if (!w) return bet
        return {
          ...bet,
          net_payout: w.net_payout,
          gross_return: w.gross_return,
          gross_profit: w.gross_profit,
          service_fee: w.service_fee,
          service_fee_pct: w.service_fee_pct,
          payout: w.net_payout,
        }
      }
      return {
        ...state,
        liveBets: state.liveBets.map(applySettlement),
        myBet: state.myBet ? applySettlement(state.myBet) : undefined,
      }
    }

    case 'crash.bet_placed': {
      const bet: CrashBet = {
        id: `${payload.user_id}:${roundId}`,
        round_id: roundId ?? '',
        user_id: payload.user_id ?? '',
        display_name: payload.display_name ?? 'Player',
        stake: payload.stake ?? 0,
        status: 'active',
        payout: 0,
        created_at: new Date().toISOString(),
      }
      const updatedRound = state.round
        ? {
            ...state.round,
            bet_count: payload.bet_count ?? state.round.bet_count,
            prize_pool: (payload as any).prize_pool ?? state.round.prize_pool,
          }
        : state.round
      return { ...state, round: updatedRound, liveBets: upsertBet(state.liveBets, bet) }
    }

    case 'crash.bet_cashed_out': {
      const existing = state.liveBets.find((b) => b.user_id === payload.user_id)
      const bet: CrashBet = {
        id: existing?.id ?? `${payload.user_id}:${roundId}`,
        round_id: roundId ?? '',
        user_id: payload.user_id ?? '',
        display_name: payload.display_name ?? existing?.display_name ?? 'Player',
        stake: payload.stake ?? existing?.stake ?? 0,
        status: 'cashed_out',
        cashout_multiplier: payload.multiplier,
        payout: payload.payout ?? 0,
        created_at: existing?.created_at ?? new Date().toISOString(),
      }
      return { ...state, liveBets: upsertBet(state.liveBets, bet) }
    }

    default:
      return state
  }
}

function reducer(state: GameState, action: GameAction): GameState {
  switch (action.type) {
    case 'snapshot': {
      const { data } = action
      const round = data.round
      // The snapshot is authoritative: it is what a fresh load or a reconnect sees.
      return {
        round,
        state: round?.state ?? 'unknown',
        multiplier: data.multiplier ?? 1,
        crashMultiplier: round && round.state !== 'RUNNING' ? round.crash_multiplier : undefined,
        myBet: data.my_bet,
        liveBets: data.live_bets ?? [],
        recent: data.recent ?? [],
        previousRound: data.previous_round,
        startedAtMs: toMs(round?.started_at),
        offsetMs: action.offsetMs,
      }
    }
    case 'event': {
      const next = reduceEvent(state, action.event)
      return action.offsetMs === undefined ? next : { ...next, offsetMs: action.offsetMs }
    }
    case 'my_bet':
      return { ...state, myBet: action.bet, liveBets: upsertBet(state.liveBets, action.bet) }
    default:
      return state
  }
}

/** Multiplier the server would show at the given moment. Display only. */
const multiplierAt = (startedAtMs: number | undefined, nowMs: number, growth: number, cap: number) => {
  if (!startedAtMs || !growth) return 1
  const elapsed = Math.max(0, (nowMs - startedAtMs) / 1000)
  return Math.min(Math.exp(growth * elapsed), cap)
}

const newIdempotencyKey = () => {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `bet-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

const formatMoney = (value: number, currency: string) =>
  `${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${currency}`

const crashChipClass = (value: number) =>
  value >= 10
    ? 'bg-amber-500/15 text-amber-600 border-amber-500/30'
    : value >= 2
      ? 'bg-green-500/15 text-green-600 border-green-500/30'
      : 'bg-rose-500/15 text-rose-600 border-rose-500/30'

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function Component() {
  const { t } = useTranslation()
  const { token } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()

  const [game, dispatch] = useReducer(reducer, initialGame)
  const [amount, setAmount] = useState('50')
  const [autoCashout, setAutoCashout] = useState('')
  const [nowMs, setNowMs] = useState(() => Date.now())
  // Interpolated multiplier for the current round, refreshed on every animation
  // frame. Keyed by round so a new round never inherits the previous one's curve.
  const [liveTick, setLiveTick] = useState<{ roundId: string; value: number }>({ roundId: '', value: 1 })

  const keyRef = useRef<string | null>(null)

  // The animation loop needs the clock offset without being torn down each time
  // a tick arrives, so it reads it from a ref that mirrors the reducer state.
  const offsetRef = useRef(0)
  useEffect(() => {
    offsetRef.current = game.offsetMs
  }, [game.offsetMs])

  const stateQuery = useQuery({
    queryKey: ['crash-state'],
    queryFn: crashApi.state,
    // The stream is the primary source of truth; poll only as a fallback while
    // it is unhealthy so a dropped connection can never freeze the screen.
    refetchInterval: 10000,
  })

  const walletQuery = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance })

  const myBetsQuery = useQuery({
    queryKey: ['crash-my-bets'],
    queryFn: () => crashApi.myBets(10, 0),
  })

  const config = stateQuery.data?.config
  const currency = config?.currency ?? 'ETB'

  const refetchAll = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['crash-state'] })
    void queryClient.invalidateQueries({ queryKey: ['crash-my-bets'] })
    void queryClient.invalidateQueries({ queryKey: ['wallet'] })
  }, [queryClient])

  // Seed from the authoritative snapshot whenever a fresh one arrives.
  useEffect(() => {
    if (!stateQuery.data) return
    const offsetMs = Date.parse(stateQuery.data.server_time) - Date.now()
    dispatch({ type: 'snapshot', data: stateQuery.data, offsetMs })
  }, [stateQuery.data, stateQuery.dataUpdatedAt])

  const onEvent = useCallback(
    (event: CrashEvent) => {
      const serverTime = event.payload?.server_time
      const offsetMs = serverTime ? Date.parse(serverTime) - Date.now() : undefined
      dispatch({ type: 'event', event, offsetMs })
      if (event.type === 'crash.settled' || event.type === 'crash.p2p_settled') {
        void queryClient.invalidateQueries({ queryKey: ['crash-my-bets'] })
        void queryClient.invalidateQueries({ queryKey: ['wallet'] })
      }
      if (event.type === 'crash.round_created') {
        void queryClient.invalidateQueries({ queryKey: ['crash-state'] })
      }
    },
    [queryClient],
  )

  const { status: streamStatus } = useCrashStream({
    enabled: !!token,
    token,
    onEvent,
    onReconnect: refetchAll,
  })

  // A slow heartbeat keeps countdowns and the "is it time yet" comparisons fresh
  // even between ticks.
  useEffect(() => {
    const id = setInterval(() => setNowMs(Date.now()), 250)
    return () => clearInterval(id)
  }, [])

  // Smooth multiplier: interpolate between server ticks with the same curve the
  // server uses. This is purely cosmetic - the payout always comes from the
  // server, so a wrong clock here cannot earn anyone anything.
  const cap = config?.max_multiplier ?? 100000
  const runningRoundId = game.state === 'RUNNING' ? (game.round?.id ?? '') : ''
  const startedAtMs = game.startedAtMs
  const growthRate = config?.growth_rate
  useEffect(() => {
    if (!runningRoundId || !startedAtMs || growthRate === undefined) return
    let raf = 0
    let last = 0
    const loop = (ts: number) => {
      raf = requestAnimationFrame(loop)
      if (ts - last < 33) return
      last = ts
      const serverNow = Date.now() + offsetRef.current
      setLiveTick({ roundId: runningRoundId, value: multiplierAt(startedAtMs, serverNow, growthRate, cap) })
    }
    raf = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(raf)
  }, [runningRoundId, startedAtMs, growthRate, cap])

  const placeBet = useMutation({
    mutationFn: () => {
      // One key per logical bet: a retry after a timeout reuses it, so the
      // backend returns the original bet instead of debiting twice.
      keyRef.current ??= newIdempotencyKey()
      const auto = Number(autoCashout)
      return crashApi.placeBet(
        {
          amount: Number(amount),
          auto_cashout_multiplier: auto > 1 ? auto : undefined,
        },
        keyRef.current,
      )
    },
    onSuccess: (bet) => {
      keyRef.current = null
      dispatch({ type: 'my_bet', bet })
      void queryClient.invalidateQueries({ queryKey: ['wallet'] })
      toast({ title: t('crash.toast.betPlaced'), description: formatMoney(bet.stake, currency), type: 'success' })
    },
    onError: (error: Error) => {
      toast({ title: t('crash.toast.betFailed'), description: error.message, type: 'error' })
    },
  })

  const cashOut = useMutation({
    mutationFn: () => crashApi.cashOut(),
    onSuccess: (bet) => {
      dispatch({ type: 'my_bet', bet })
      void queryClient.invalidateQueries({ queryKey: ['wallet'] })
      toast({
        title: t('crash.toast.cashedOut'),
        description: `${bet.cashout_multiplier?.toFixed(2)}x · ${formatMoney(bet.payout, currency)}`,
        type: 'success',
      })
    },
    onError: (error: Error) => {
      toast({ title: t('crash.toast.cashOutFailed'), description: error.message, type: 'error' })
      void queryClient.invalidateQueries({ queryKey: ['crash-state'] })
    },
  })

  const verify = useMutation({
    mutationFn: (roundId: string) => crashApi.verifyRound(roundId),
  })

  const autoKey = `${amount}|${autoCashout}`
  useEffect(() => {
    // A changed stake or target is a different intent: use a fresh key.
    keyRef.current = null
  }, [autoKey])

  const balance = walletQuery.data?.balance ?? 0
  const minBet = config?.min_bet ?? 0
  const maxBet = config?.max_bet ?? Number.MAX_SAFE_INTEGER
  const amountValue = Number(amount)
  const amountValid = Number.isFinite(amountValue) && amountValue >= minBet && amountValue <= maxBet
  const enoughFunds = amountValue <= balance

  const state = game.state
  const myBet = game.myBet
  const roundId = game.round?.id ?? ''
  // The stake can be edited until the round starts (no bet placed yet).
  const canBet = state === 'WAITING' || state === 'BETTING'
  const serverNowMs = nowMs + game.offsetMs
  // The server only accepts bets between betting_starts_at and betting_ends_at
  // while the round is in BETTING, so the button mirrors exactly that window
  // instead of offering a bet the server would refuse.
  const bettingOpen =
    !!game.round &&
    state === 'BETTING' &&
    serverNowMs >= Date.parse(game.round.betting_starts_at) &&
    serverNowMs < Date.parse(game.round.betting_ends_at)

  // Countdown: to the betting deadline while betting, to the next round otherwise.
  const countdownMs = useMemo(() => {
    if (state === 'WAITING' && game.round) return Math.max(0, Date.parse(game.round.betting_starts_at) - serverNowMs)
    if ((state === 'BETTING' || state === 'unknown') && game.round)
      return Math.max(0, Date.parse(game.round.betting_ends_at) - serverNowMs)
    return 0
  }, [serverNowMs, state, game.round])

  // Display multiplier: the live curve while running (falling back to the last
  // server tick until the first animation frame), otherwise the authoritative
  // value carried by the snapshot or the crash event.
  const display =
    state === 'RUNNING'
      ? liveTick.roundId === roundId
        ? liveTick.value
        : game.multiplier
      : (game.crashMultiplier ?? game.multiplier)

  const players = game.liveBets.length
  const progress = Math.min(1, Math.log(Math.max(display, 1)) / Math.log(10))

  const statusLabel = useMemo(() => {
    switch (state) {
      case 'RUNNING':
        return t('crash.status.flying')
      case 'CRASHED':
        return t('crash.status.crashed')
      default:
        return t('crash.status.nextRound')
    }
  }, [state, t])

  const primaryAction = () => {
    if (myBet?.status === 'active' && state === 'RUNNING') {
      return (
        <Button
          size="lg"
          variant="destructive"
          className="w-full text-base font-bold"
          onClick={() => cashOut.mutate()}
          disabled={cashOut.isPending}
        >
          {cashOut.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
          {t('crash.cashOut')} {(display).toFixed(2)}x
        </Button>
      )
    }
    if (myBet?.status === 'cashed_out') {
      return (
        <Button size="lg" variant="secondary" className="w-full" disabled>
          <BadgeCheck className="h-4 w-4 text-green-500" />
          {t('crash.cashedOutAt', { multiplier: myBet.cashout_multiplier?.toFixed(2) })}
        </Button>
      )
    }
    if (myBet?.status === 'lost') {
      return (
        <Button size="lg" variant="outline" className="w-full" disabled>
          {t('crash.lost')}
        </Button>
      )
    }
    return (
      <Button
        size="lg"
        className="w-full text-base font-bold"
        onClick={() => placeBet.mutate()}
        disabled={!bettingOpen || !amountValid || !enoughFunds || placeBet.isPending}
      >
        {placeBet.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
        {bettingOpen
          ? t('crash.placeBet')
          : state === 'RUNNING'
            ? t('crash.status.inProgress')
            : t('crash.status.bettingClosed')}
      </Button>
    )
  }

  // P2P derived values
  const p2pMode = game.round?.p2p_mode ?? true
  const prizePool = game.round?.prize_pool ?? 0
  const serviceFeePct = game.round?.service_fee_percent ?? config?.service_fee_percent ?? 5
  const minParticipants = game.round?.min_participants ?? config?.min_participants ?? 2

  return (
    <div className="mx-auto max-w-6xl space-y-4 p-4">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('crash.title')}</h1>
          <p className="text-xs text-muted-foreground">{t('crash.subtitle')}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline">
            {t('crash.round')} #{game.round?.round_number ?? '—'}
          </Badge>
          <Badge variant="outline" className="gap-1">
            <Users className="h-3 w-3" />
            {t('crash.players', { count: players })}
            {minParticipants > 1 && (
              <span className="text-muted-foreground">/{minParticipants} min</span>
            )}
          </Badge>
          {p2pMode && (
            <Badge variant="outline" className="gap-1 border-green-500/40 text-green-600">
              <CircleDollarSign className="h-3 w-3" />
              {t('crash.prizePool')}: {formatMoney(prizePool, currency)}
            </Badge>
          )}
          {streamStatus === 'reconnecting' || streamStatus === 'connecting' ? (
            <Badge variant="warning" className="gap-1">
              <WifiOff className="h-3 w-3" />
              {t('crash.reconnecting')}
            </Badge>
          ) : (
            <Badge variant="success" className="gap-1">
              <span className="h-1.5 w-1.5 rounded-full bg-green-600" />
              {t('crash.live')}
            </Badge>
          )}
        </div>
      </div>

      {/* P2P fee disclosure banner — shown before the user joins */}
      {p2pMode && !myBet && (
        <div className="flex items-start gap-2 rounded-lg border border-blue-500/30 bg-blue-500/5 px-4 py-3 text-xs text-blue-700 dark:text-blue-300">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>
            <strong>Peer-to-peer game.</strong> Your stake joins a shared prize pool.
            Winners split the pool proportionally to their cash-out multiplier.
            A <strong>{serviceFeePct.toFixed(2)}% service fee</strong> is deducted only from a winner's
            positive profit — losers and break-even players pay no fee.
          </span>
        </div>
      )}

      {stateQuery.isError && (
        <Card className="border-destructive/40">
          <CardContent className="flex items-center gap-2 p-4 text-sm text-destructive">
            <AlertTriangle className="h-4 w-4" />
            {(stateQuery.error as Error)?.message ?? t('errors.serverError')}
            <Button size="sm" variant="outline" onClick={refetchAll}>
              {t('common.retry')}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* min-w-0 keeps the grid columns shrinkable on narrow screens: without it
          a wide child (the flight stage) forces the whole page to scroll. */}
      <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
        {/* ── Game area ── */}
        <div className="min-w-0 space-y-4">
          <FlightStage
            state={state}
            multiplier={display}
            crashMultiplier={game.crashMultiplier}
            countdownMs={countdownMs}
            progress={progress}
            label={statusLabel}
            countdownLabel={state === 'WAITING' ? t('crash.status.startingIn') : t('crash.status.bettingEndsIn')}
          />

          {/* History strip */}
          <Card className="min-w-0">
            <CardHeader className="flex-row items-center justify-between space-y-0 py-3">
              <CardTitle className="flex items-center gap-2 text-sm">
                <History className="h-4 w-4" />
                {t('crash.history')}
              </CardTitle>
            </CardHeader>
            <CardContent className="px-6 pb-4">
              {game.recent.length === 0 ? (
                <p className="text-xs text-muted-foreground">{t('crash.noHistory')}</p>
              ) : (
                <div className="flex flex-wrap gap-1.5">
                  {game.recent.slice(0, 20).map((round) => (
                    <span
                      key={round.id}
                      className={`rounded-full border px-2.5 py-1 font-mono text-xs font-semibold ${crashChipClass(
                        round.crash_multiplier,
                      )}`}
                      title={`#${round.round_number}`}
                    >
                      {round.crash_multiplier.toFixed(2)}x
                    </span>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Live multiplayer table */}
          <Card>
            <CardHeader className="flex-row items-center justify-between space-y-0 py-3">
              <CardTitle className="flex items-center gap-2 text-sm">
                <CircleDollarSign className="h-4 w-4" />
                {t('crash.liveTable')}
              </CardTitle>
              <div className="flex flex-col items-end gap-0.5">
                <span className="text-xs text-muted-foreground">
                  {t('crash.totalOnTable', {
                    amount: formatMoney(game.liveBets.reduce((sum, b) => sum + b.stake, 0), currency),
                  })}
                </span>
                {p2pMode && prizePool > 0 && (
                  <span className="text-xs font-semibold text-green-600">
                    Pool: {formatMoney(prizePool, currency)}
                  </span>
                )}
              </div>
            </CardHeader>
            <CardContent className="px-0 pb-2">
              {game.liveBets.length === 0 ? (
                <p className="px-6 pb-4 text-xs text-muted-foreground">{t('crash.noPlayers')}</p>
              ) : (
                <div className="max-h-80 overflow-y-auto">
                  <table className="w-full text-sm">
                    <thead className="sticky top-0 bg-card text-xs uppercase tracking-wide text-muted-foreground">
                      <tr>
                        <th className="px-6 py-2 text-left font-medium">{t('crash.table.player')}</th>
                        <th className="px-3 py-2 text-right font-medium">{t('crash.table.stake')}</th>
                        <th className="px-3 py-2 text-right font-medium">{t('crash.table.cashOut')}</th>
                        <th className="px-6 py-2 text-right font-medium">
                          {p2pMode ? 'Net' : t('crash.table.payout')}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {[...game.liveBets]
                        .sort((a, b) => b.stake - a.stake)
                        .map((bet) => (
                          <tr key={bet.id} className="border-t border-border/60">
                            <td className="max-w-[10rem] truncate px-6 py-2 font-medium">
                              {bet.display_name}
                              {myBet && bet.user_id === myBet.user_id && (
                                <span className="ml-1 text-xs text-primary">({t('crash.you')})</span>
                              )}
                            </td>
                            <td className="px-3 py-2 text-right font-mono tabular-nums">
                              {bet.stake.toFixed(2)}
                            </td>
                            <td className="px-3 py-2 text-right font-mono tabular-nums">
                              {bet.status === 'cashed_out' ? (
                                <span className="text-green-600">
                                  {bet.cashout_multiplier?.toFixed(2)}x
                                </span>
                              ) : bet.status === 'lost' ? (
                                <span className="text-destructive">{t('crash.lost')}</span>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </td>
                            <td className="px-6 py-2 text-right font-mono tabular-nums">
                              {bet.status === 'cashed_out' ? (
                                bet.net_payout != null && bet.net_payout > 0 ? (
                                  <span className="font-semibold text-green-600">
                                    +{bet.net_payout.toFixed(2)}
                                  </span>
                                ) : p2pMode ? (
                                  <span className="text-muted-foreground text-[10px]">settling…</span>
                                ) : (
                                  <span className="font-semibold text-green-600">
                                    +{bet.payout.toFixed(2)}
                                  </span>
                                )
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </td>
                          </tr>
                        ))}
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* ── Bet panel ── */}
        <div className="space-y-4">
          <Card>
            <CardHeader className="py-3">
              <CardTitle className="text-sm">{t('crash.betPanel')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between rounded-lg bg-accent/40 px-3 py-2 text-sm">
                <span className="text-muted-foreground">{t('crash.balance')}</span>
                <span className="font-semibold">{formatMoney(balance, currency)}</span>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="crash-amount">{t('crash.amount')}</Label>
                <Input
                  id="crash-amount"
                  type="number"
                  inputMode="decimal"
                  min={minBet}
                  max={maxBet}
                  step="0.01"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  disabled={!canBet || !!myBet}
                />
                <div className="flex flex-wrap gap-1.5">
                  {[minBet, 50, 100, 500].map((preset, index) => (
                    <Button
                      key={`${preset}-${index}`}
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={!canBet || !!myBet}
                      onClick={() => setAmount(String(preset))}
                    >
                      {preset}
                    </Button>
                  ))}
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={!canBet || !!myBet}
                    onClick={() => setAmount(String(Math.floor(balance * 100) / 100))}
                  >
                    {t('crash.max')}
                  </Button>
                </div>
                {!amountValid && (
                  <p className="text-xs text-destructive">
                    {t('crash.hint.range', { min: minBet.toFixed(2), max: maxBet.toFixed(2) })}
                  </p>
                )}
                {amountValid && !enoughFunds && (
                  <p className="text-xs text-destructive">{t('crash.hint.insufficient')}</p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="crash-auto">{t('crash.autoCashOut')}</Label>
                <Input
                  id="crash-auto"
                  type="number"
                  inputMode="decimal"
                  min="1.01"
                  step="0.01"
                  placeholder={t('crash.autoCashOutHint')}
                  value={autoCashout}
                  onChange={(e) => setAutoCashout(e.target.value)}
                  disabled={!canBet || !!myBet}
                />
              </div>

              {myBet?.status === 'active' && (
                <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">{t('crash.yourStake')}</span>
                    <span className="font-mono">{formatMoney(myBet.stake, currency)}</span>
                  </div>
                  {p2pMode ? (
                    <>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">Prize pool</span>
                        <span className="font-mono font-semibold text-green-600">
                          {formatMoney(prizePool, currency)}
                        </span>
                      </div>
                      <div className="flex justify-between text-[10px] text-muted-foreground">
                        <span>Fee on profit</span>
                        <span>{serviceFeePct.toFixed(2)}% (winners only)</span>
                      </div>
                    </>
                  ) : (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">{t('crash.potential')}</span>
                      <span className="font-mono font-semibold text-green-600">
                        {formatMoney(myBet.stake * Math.max(display, 1), currency)}
                      </span>
                    </div>
                  )}
                </div>
              )}

              {/* Show P2P settlement breakdown once the round is settled */}
              {myBet?.status === 'cashed_out' && myBet.net_payout != null && myBet.net_payout > 0 && p2pMode && (
                <div className="rounded-lg border border-green-500/40 bg-green-500/5 px-3 py-2 text-xs">
                  <p className="mb-1 font-semibold text-green-700 dark:text-green-400">Settlement</p>
                  <div className="space-y-0.5">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Gross return</span>
                      <span className="font-mono">{formatMoney(myBet.gross_return ?? 0, currency)}</span>
                    </div>
                    {(myBet.service_fee ?? 0) > 0 && (
                      <div className="flex justify-between text-destructive/80">
                        <span>Service fee ({serviceFeePct.toFixed(2)}% of profit)</span>
                        <span className="font-mono">−{formatMoney(myBet.service_fee ?? 0, currency)}</span>
                      </div>
                    )}
                    <div className="flex justify-between font-semibold text-green-700 dark:text-green-400">
                      <span>Net payout</span>
                      <span className="font-mono">+{formatMoney(myBet.net_payout, currency)}</span>
                    </div>
                  </div>
                </div>
              )}

              {primaryAction()}

              <p className="flex items-start gap-1.5 text-[11px] leading-snug text-muted-foreground">
                <ShieldCheck className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                {t('crash.serverNote')}
              </p>
            </CardContent>
          </Card>

          {/* Fairness */}
          {game.previousRound && (
            <Card>
              <CardHeader className="flex-row items-center justify-between space-y-0 py-3">
                <CardTitle className="flex items-center gap-2 text-sm">
                  <BadgeCheck className="h-4 w-4" />
                  {t('crash.fairness')}
                </CardTitle>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => verify.mutate(game.previousRound!.id)}
                  disabled={verify.isPending}
                >
                  {verify.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : t('crash.verify')}
                </Button>
              </CardHeader>
              <CardContent className="space-y-2 px-6 pb-4 text-xs">
                <div className="flex justify-between gap-2">
                  <span className="text-muted-foreground">{t('crash.commitment')}</span>
                  <span className="truncate font-mono">{game.previousRound.server_seed_hash.slice(0, 16)}…</span>
                </div>
                {verify.data ? (
                  <VerificationResult proof={verify.data} currency={currency} />
                ) : verify.isError ? (
                  <p className="text-destructive">{(verify.error as Error).message}</p>
                ) : (
                  <p className="flex items-start gap-1.5 text-muted-foreground">
                    <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    {t('crash.fairnessHint')}
                  </p>
                )}
              </CardContent>
            </Card>
          )}

          {/* My bets */}
          <Card>
            <CardHeader className="py-3">
              <CardTitle className="flex items-center gap-2 text-sm">
                <History className="h-4 w-4" />
                {t('crash.myBets')}
              </CardTitle>
            </CardHeader>
            <CardContent className="px-0 pb-2 text-xs">
              {myBetsQuery.isLoading ? (
                <p className="px-6 pb-4 text-muted-foreground">{t('common.loading')}</p>
              ) : (myBetsQuery.data?.bets.length ?? 0) === 0 ? (
                <p className="px-6 pb-4 text-muted-foreground">{t('crash.noBets')}</p>
              ) : (
                <ul className="divide-y divide-border/60">
                  {myBetsQuery.data!.bets.map((bet) => (
                    <li key={bet.id} className="flex items-center justify-between gap-2 px-6 py-2">
                      <span className="font-mono">{formatMoney(bet.stake, currency)}</span>
                      {bet.status === 'cashed_out' ? (
                        <span className="text-green-600">
                          {bet.cashout_multiplier?.toFixed(2)}x · +{bet.payout.toFixed(2)}
                        </span>
                      ) : bet.status === 'lost' ? (
                        <span className="text-destructive">{t('crash.lost')}</span>
                      ) : (
                        <span className="text-muted-foreground">{t('crash.status.inProgress')}</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}

/** Renders the public commit-reveal proof returned by the server. */
function VerificationResult({ proof, currency }: { proof: CrashVerification; currency: string }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-1 rounded-md bg-muted/50 p-2">
      <div className="flex items-center gap-1.5 font-semibold">
        {proof.verified ? (
          <>
            <BadgeCheck className="h-3.5 w-3.5 text-green-600" />
            <span className="text-green-600">{t('crash.proofValid')}</span>
          </>
        ) : (
          <>
            <AlertTriangle className="h-3.5 w-3.5 text-destructive" />
            <span className="text-destructive">{t('crash.proofInvalid')}</span>
          </>
        )}
      </div>
      <p className="font-mono">
        {t('crash.proofClaimed')}: {proof.crash_multiplier.toFixed(2)}x ·{' '}
        {t('crash.proofRecomputed')}: {proof.recomputed_multiplier.toFixed(2)}x
      </p>
      <p className="break-all font-mono text-[10px] text-muted-foreground">
        {t('crash.proofSeed')}: {proof.server_seed}
      </p>
      <p className="text-[10px] text-muted-foreground">{proof.algorithm}</p>
      <p className="text-[10px] text-muted-foreground">
        {t('crash.proofCurrencyNote', { currency })}
      </p>
    </div>
  )
}
