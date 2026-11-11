import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import {
  Users,
  Swords,
  Clock,
  Copy,
  Check,
  Flag,
  ArrowLeft,
  X,
  Wifi,
  WifiOff,
  Loader2,
  Send,
  Share2,
  AlertTriangle,
  Trophy,
  Coins,
  History,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { walletApi } from '@/features/wallet/api'
import { formatCurrency } from '@/lib/utils'
import { gameName } from '@/features/p2p/matchHistory'
import {
  buildInviteUrl,
  buildWebInviteUrl,
  buildInviteMessage,
  shareViaTelegram,
  shareLinkOrCopy,
  copyToClipboard,
} from '@/lib/telegram'
import { useP2PStream, type P2PEvent } from '@/features/p2p/useP2PStream'
// import { consumeRematchRequest } from '@/features/p2p/rematchEvents'
import { hasMatchVersionGap, mergeMatchSnapshot } from '@/features/p2p/matchState'
import { deriveMatchConnection } from '@/features/p2p/matchConnection'
import { TicTacToeBoard } from '@/features/p2p/TicTacToeBoard'
import { PlayerOneMarker, PlayerTwoMarker, TicTacToeMarker } from '@/features/p2p/TicTacToeMarkers'
import { ConnectFourToken } from '@/features/p2p/ConnectFourToken'
import { DamaPiece } from '@/features/p2p/DamaPiece'
import { seatForPlayerIndex } from '@/features/p2p/ticTacToe'
import { ConnectFourBoard } from '@/features/p2p/ConnectFourBoard'
import { RockPaperScissorsBoard } from '@/features/p2p/RockPaperScissorsBoard'
import { DamaBoard } from '@/features/p2p/DamaBoard'
import { type DamaBoardState } from '@/features/p2p/dama'
import { applyOptimisticMove, buildOptimisticMove, type OptimisticMove } from '@/features/p2p/optimisticMove'
import { useGameSounds } from '@/features/p2p/useGameSounds'
import { SoundToggle } from '@/features/p2p/SoundToggle'
import { MatchHistoryList, PaidMetric } from '@/features/p2p/MatchHistory'
import {
  RECENT_HISTORY_LIMIT,
  isTerminalMatch as isTerminalMatchRecord,
} from '@/features/p2p/matchHistory'
import {
  useGameOutcomeController,
  GameOutcomeOverlay,
  DecisiveBanner,
  VictoryConfetti,
} from '@/features/p2p/outcome'
import type { P2PMatch } from '@/types/api'

// Surfaces the API's error message (axios error shape) without widening to any.
function apiErrorMessage(err: unknown, t?: TFunction): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return t ? t('errors.somethingWentWrong', 'Something went wrong') : 'Something went wrong'
}

function MatchHistoryModal({
  matches,
  isLoading,
  isError,
  onRetry,
  onClose,
  currentUserId,
}: {
  matches: P2PMatch[]
  isLoading: boolean
  isError: boolean
  onRetry: () => void
  onClose: () => void
  currentUserId?: string
}) {
  const { t } = useTranslation()
  return (
    <div
      className="fixed inset-0 z-50 flex items-stretch justify-center bg-background/80 backdrop-blur-sm sm:items-center sm:p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="match-history-title"
      onClick={onClose}
    >
      <Card
        className="flex h-screen h-[100dvh] w-full max-w-2xl flex-col overflow-hidden rounded-none border-0 bg-card pb-[env(safe-area-inset-bottom)] pt-[env(safe-area-inset-top)] shadow-2xl sm:h-auto sm:max-h-[85vh] sm:rounded-2xl sm:border sm:border-border/80 sm:pb-0 sm:pt-0"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-border/70 px-4 py-3.5 sm:px-5">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <History className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <h2 id="match-history-title" className="truncate text-base font-bold text-foreground">
                {t('p2p.historyModal.title', 'Recent Match History')}
              </h2>
              <p className="truncate text-xs text-muted-foreground">
                {t('p2p.historyModal.subtitle', 'Your latest results and settlement records')}
              </p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {!isLoading && !isError && matches.length > 0 && (
              <Badge
                variant="outline"
                className="hidden text-[10px] font-semibold sm:inline-flex"
              >
                {t('p2p.historyModal.recentBadge', { count: matches.length, defaultValue: `${matches.length} recent` })}
              </Badge>
            )}
            <Button
              variant="ghost"
              size="icon"
              onClick={onClose}
              autoFocus
              aria-label={t('p2p.historyModal.close', 'Close match history')}
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto overscroll-contain p-4 sm:p-5">
          <MatchHistoryList
            matches={matches}
            isLoading={isLoading}
            isError={isError}
            onRetry={onRetry}
            currentUserId={currentUserId}
          />
        </div>
      </Card>
    </div>
  )
}

export function Component() {
  const { t } = useTranslation()
  const { id: matchId } = useParams<{ id: string }>()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  // const handledRematchRequestsRef = useRef(new Set<string>())
  const openedRematchIdRef = useRef<string | null>(null)

  const openRematch = useCallback(
    (nextMatchId: string) => {
      if (!nextMatchId || nextMatchId === matchId || openedRematchIdRef.current === nextMatchId) return
      openedRematchIdRef.current = nextMatchId
      navigate(`/games/match/${nextMatchId}`)
    },
    [matchId, navigate],
  )

  const [copied, setCopied] = useState(false)
  const [copiedLink, setCopiedLink] = useState(false)
  const [secondsLeft, setSecondsLeft] = useState<number>(0)
  const [waitingSecondsLeft, setWaitingSecondsLeft] = useState<number>(0)
  const [disconnectSecondsLeft, setDisconnectSecondsLeft] = useState<number>(0)
  const [resignDialogOpen, setResignDialogOpen] = useState(false)
  const [cancelDialogOpen, setCancelDialogOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  // A move overlay belongs to one match only. Tagging it with its match lets
  // navigation drop it without an effect (and without touching a newer move).
  const [pendingMove, setPendingMove] = useState<{ matchId: string; move: OptimisticMove } | null>(null)
  const moveInFlightRef = useRef<string | null>(null)
  const activePendingMove =
    pendingMove && pendingMove.matchId === matchId ? pendingMove.move : null

  // Fetch initial match state
  const {
    data: match,
    isLoading,
    isError,
    error,
    refetch,
    dataUpdatedAt,
  } = useQuery<P2PMatch>({
    queryKey: ['p2p-match', matchId],
    queryFn: async () => {
      const key = ['p2p-match', matchId] as const
      const incoming = await p2pApi.getMatchState(matchId!)
      return mergeMatchSnapshot(queryClient.getQueryData<P2PMatch>(key), incoming)
    },
    enabled: !!matchId,
    // The match channel pushes state changes; a fallback poll below keeps the
    // board in sync while that stream is down (see "resyncFromServer").
  })

  // Paid matches need the wallet balance for the paid match summary. The
  // balance is only fetched while a staked match is on screen.
  const isPaidMatch = match?.competition_mode === 'earn'
  const { data: wallet } = useQuery({
    queryKey: ['wallet'],
    queryFn: walletApi.balance,
    enabled: !!isPaidMatch,
    staleTime: 0,
  })

  // Reuse the same cached history query as the lobby so opening a match does
  // not duplicate the endpoint or lose the already-loaded history. History is
  // only requested once the user opens it from the match page.
  const {
    data: matchHistory,
    isLoading: historyLoading,
    isError: historyError,
    refetch: refetchHistory,
  } = useQuery({
    queryKey: ['p2p-my-matches'],
    queryFn: () => p2pApi.listMyMatches(20, 0),
    enabled: !!user && historyOpen,
  })

  const applyMatchSnapshot = useCallback(
    (incoming: P2PMatch) => {
      if (incoming.id !== matchId) return
      const key = ['p2p-match', matchId] as const
      const current = queryClient.getQueryData<P2PMatch>(key)
      const missedVersions = hasMatchVersionGap(current, incoming)
      queryClient.setQueryData<P2PMatch>(key, (cached) => mergeMatchSnapshot(cached, incoming))
      // The event already contains a usable full snapshot. Refetch once when
      // its sequence proves that another committed state was missed.
      if (missedVersions) void refetch()
    },
    [matchId, queryClient, refetch],
  )

  // Web Audio sound effects manager
  const { isMuted, toggleMute } = useGameSounds({
    match,
    userId: user?.id,
    secondsLeft,
    timerThreshold: 5,
  })

  // Staged decisive event & outcome controller
  const {
    outcome,
    phase: outcomePhase,
    isEffectsActive,
    isAnnounced,
    isReviewingBoard,
    skipToAnnouncement,
    toggleReviewBoard,
  } = useGameOutcomeController({
    match,
    currentUserId: user?.id,
  })

  // Handle realtime events
  const handleEvent = useCallback(
    (event: P2PEvent) => {
      // SSE payloads are dynamic — cast to a loose record for field access.
      const payload = event.payload as Record<string, unknown>
      // State-bearing events carry a view built for *this* player under `state`
      // (see P2PService.publishMatchState). Older/partial payloads without a
      // `state` field fall through to an authoritative refetch.
      const eventState = (payload?.state ?? null) as P2PMatch | null
      const currentEventState = eventState && eventState.id === matchId ? eventState : null
      const serverTiming = payload?.server_timing_ms
      if (
        event.type === 'p2p.move_made' &&
        currentEventState &&
        typeof serverTiming === 'object' &&
        serverTiming !== null
      ) {
        performance.mark(`p2p-move:${matchId}:${currentEventState.state_version ?? 'unknown'}`, {
          detail: {
            server_timing_ms: serverTiming,
            server_delivery_latency_ms: event.server_delivery_latency_ms,
            event_payload_bytes: new TextEncoder().encode(JSON.stringify(event.payload)).byteLength,
          },
        })
      }
      if (event.type === 'p2p.move_made' && currentEventState) {
        applyMatchSnapshot(currentEventState)
      } else if (
        event.type === 'p2p.match_started' ||
        // A round break ends with the next round's board and turn; the event
        // itself only announces it, so fetch the authoritative state.
        event.type === 'p2p.round_started' ||
        event.type === 'p2p.turn_timeout'
      ) {
        // match_started carries the started match; the other two are signals to
        // refetch the canonical (possibly settled) state.
        if (event.type === 'p2p.match_started' && currentEventState) {
          applyMatchSnapshot(currentEventState)
        } else if (event.type === 'p2p.round_started') {
          // The round transition carries only metadata (no player-specific
          // snapshot), so read the authoritative version and merge it into the
          // cache immediately. A direct fetch also avoids reusing an unrelated
          // pending query refetch started during the winning-line reveal.
          void p2pApi.getMatchState(matchId!).then(applyMatchSnapshot).catch(() => refetch())
        } else {
          refetch()
        }
      } else if (event.type === 'p2p.match_completed') {
        if (currentEventState) applyMatchSnapshot(currentEventState)
        else refetch()
      } else if (event.type === 'p2p.player_disconnected') {
        // Payload contains disconnect_user_id and disconnect_seconds_left.
        // Apply a full player-specific view immediately; older events trigger
        // an authoritative refetch through the version-gap check.
        if (currentEventState) applyMatchSnapshot(currentEventState)
        else refetch()
      } else if (event.type === 'p2p.player_reconnected') {
        // Payload contains the full match state under `state`.
        if (currentEventState) {
          applyMatchSnapshot(currentEventState)
        } else {
          refetch()
        }
      } else if (event.type === 'p2p.rematch_requested') {
        // The requester is told separately; only the opponent needs a nudge.
        const eventMatchId = payload?.match_id
        // const requesterId = payload?.user_id
        if (eventMatchId === matchId && currentEventState) {
          // The event is the server acknowledgement. Apply it now so the
          // requester sees Waiting and the opponent sees Accept immediately.
          applyMatchSnapshot(currentEventState)
        }
        // if (
        //   eventMatchId === matchId &&
        //   requesterId !== user?.id &&
        //   consumeRematchRequest(handledRematchRequestsRef.current, eventMatchId, requesterId)
        // ) {
        //   toast({
        //     title: 'Rematch requested',
        //     description: 'Your opponent wants to play again.',
        //     type: 'info',
        //   })
        // }
        if (!(eventMatchId === matchId && currentEventState)) refetch()
      } else if (
        event.type === 'p2p.rematch_created' &&
        payload?.match_id === matchId &&
        typeof payload?.next_match_id === 'string'
      ) {
        // toast({ title: 'Rematch started', description: 'Both players agreed — good luck!', type: 'success' })
        openRematch(payload.next_match_id)
      } else if (event.type === 'p2p.match_cancelled') {
        toast({
          title: t('p2p.match.cancelledToast', 'Invitation cancelled'),
          description: t('p2p.match.roomClosed', 'The room has been closed.'),
          type: 'info',
        })
        refetch()
      } else {
        refetch()
      }
    },
    [matchId, applyMatchSnapshot, refetch, toast, openRematch, t]
  )

  // Reconnect mutation: when the SSE stream recovers from a disconnection,
  // notify the server so it clears the grace-period countdown. If the grace
  // period already expired (410 Gone) just refetch to show the final result.
  const reconnectMutation = useMutation({
    mutationFn: () => p2pApi.reconnectToMatch(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
    },
    onError: () => {
      // 410 Gone means the match ended while we were disconnected — refetch
      // to pick up the terminal state.
      refetch()
    },
  })

  const {
    isLive,
    status: streamStatus,
    retry: retryStream,
  } = useP2PStream({
    matchId,
    token,
    enabled: !!matchId && !!token,
    onEvent: handleEvent,
    onReconnect: () => {
      // Always refetch the canonical state on stream reconnect.
      refetch()
      // If a match is in progress and we had been disconnected, tell the server
      // so it clears the grace-period deadline.
      if (match?.status === 'in_progress' && !reconnectMutation.isPending) {
        reconnectMutation.mutate()
      }
    },
  })

  // ── Authoritative-state fallback ────────────────────────────────────────────
  // A dropped stream used to mean a frozen board: the server kept advancing the
  // match while this client showed stale state, and the turn clock then forfeited
  // the player who never saw their turn. Whenever the stream is not live — or a
  // live stream has gone quiet mid-match — poll the authoritative snapshot.
  const liveRef = useRef(isLive)
  const matchStatusRef = useRef<string | undefined>(match?.status)
  const updatedAtRef = useRef<number>(dataUpdatedAt)
  const lastFallbackPollRef = useRef(0)
  // Mirror the latest render's inputs into the refs the poll timer reads.
  // Done in an effect (not during render) so a render never mutates state it
  // is reading.
  useEffect(() => {
    liveRef.current = isLive
    matchStatusRef.current = match?.status
    updatedAtRef.current = dataUpdatedAt
  })

  const isTerminalMatch = !!match && ['completed', 'tied', 'forfeited', 'cancelled'].includes(match.status)

  // Refresh the settled history and wallet once a match reaches a terminal
  // state so the latest result and ledger references appear immediately.
  useEffect(() => {
    if (!isTerminalMatch) return
    void queryClient.invalidateQueries({ queryKey: ['p2p-my-matches'] })
    if (isPaidMatch) {
      void queryClient.invalidateQueries({ queryKey: ['wallet'] })
    }
  }, [isPaidMatch, isTerminalMatch, queryClient])

  // Staleness is observed by the poll timer below — the clock is read inside a
  // callback, never during render — and only published when it flips, so a
  // healthy page does not re-render on every tick.
  const [streamStale, setStreamStale] = useState(false)

  useEffect(() => {
    if (!matchId || isTerminalMatch) return
    lastFallbackPollRef.current = 0

    const timer = setInterval(() => {
      const inProgress = matchStatusRef.current === 'in_progress'
      const streamHealthy = liveRef.current
      const quietForMs = Date.now() - updatedAtRef.current
      const stale = inProgress && quietForMs > 10_000
      setStreamStale((prev) => (prev === stale ? prev : stale))

      const now = Date.now()
      const minimumPollGap = streamHealthy ? 10_000 : 5_000
      if ((!streamHealthy || stale) && now - lastFallbackPollRef.current >= minimumPollGap) {
        lastFallbackPollRef.current = now
        refetch()
      }
    }, 2_000)

    return () => clearInterval(timer)
  }, [matchId, isTerminalMatch, refetch])

  // Resync as soon as the tab becomes visible again or the network returns:
  // backgrounded mobile tabs are the most common way a stream silently dies.
  useEffect(() => {
    if (!matchId) return
    const resync = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
      refetch()
      if (!liveRef.current) retryStream()
    }
    const onVisibility = () => {
      if (document.visibilityState === 'visible') resync()
    }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('online', resync)
    window.addEventListener('focus', resync)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('online', resync)
      window.removeEventListener('focus', resync)
    }
  }, [matchId, refetch, retryStream])

  // Report sustained transport loss so the server's reconnection grace period
  // governs the match instead of the turn clock. Without this the player whose
  // UI was stuck simply timed out and lost.
  const reportedDisconnectRef = useRef(false)
  useEffect(() => {
    if (!matchId || match?.status !== 'in_progress') {
      reportedDisconnectRef.current = false
      return
    }
    if (isLive) {
      reportedDisconnectRef.current = false
      return
    }

    const timer = setTimeout(() => {
      if (reportedDisconnectRef.current) return
      reportedDisconnectRef.current = true
      p2pApi.disconnectFromMatch(matchId).catch(() => {
        // Offline: the grace period is only a server-side courtesy; gameplay
        // still recovers when the stream returns.
      })
    }, 5_000)

    return () => clearTimeout(timer)
  }, [matchId, match?.status, isLive])

  // Turn clock: count down to the server's absolute deadline, shifted by the
  // observed clock offset. A local "seconds left" snapshot drifts with every
  // refetch (and across devices), which is how a player ends up seeing 3 s left
  // when the server has already timed them out.
  useEffect(() => {
    if (!match || match.status !== 'in_progress') return

    const deadlineMs = match.turn_deadline_at ? new Date(match.turn_deadline_at).getTime() : null
    const serverNow = match.server_time ? new Date(match.server_time).getTime() : null
    const clockOffsetMs = serverNow !== null ? Date.now() - serverNow : 0

    const remaining = () => {
      if (deadlineMs !== null) {
        return Math.max(0, Math.ceil((deadlineMs + clockOffsetMs - Date.now()) / 1000))
      }
      return Math.max(0, match.turn_seconds_left || 0)
    }

    // First value lands in a microtask (before paint) so the effect body does
    // not set state synchronously.
    queueMicrotask(() => setSecondsLeft(remaining()))
    const timer = setInterval(() => setSecondsLeft(remaining()), 1000)
    return () => clearInterval(timer)
  }, [
    match?.current_turn_user_id,
    match?.turn_deadline_at,
    match?.turn_seconds_left,
    match?.status,
    match?.server_time,
    match,
  ])

  // Waiting room countdown using backend expiration time
  useEffect(() => {
    if (!match || match.status !== 'waiting') return
    const ttlMs = (match.match_type === 'private_room' ? 5 : 15) * 60 * 1000
    const expiresAt = match.expires_at
      ? new Date(match.expires_at).getTime()
      : new Date(match.created_at).getTime() + ttlMs

    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.floor((expiresAt - Date.now()) / 1000))
      setWaitingSecondsLeft(remaining)
    }, 1000)

    return () => clearInterval(timer)
  }, [match?.status, match?.expires_at, match?.created_at, match?.match_type, match])

  // Disconnect grace-period countdown: driven from the server-supplied
  // disconnect_seconds_left value and ticking down locally each second.
  const disconnectKey = `${match?.disconnect_user_id}_${match?.disconnect_seconds_left}`
  const [prevDisconnectKey, setPrevDisconnectKey] = useState<string>('')
  if (match?.status === 'in_progress' && match.disconnect_user_id && disconnectKey !== prevDisconnectKey) {
    setPrevDisconnectKey(disconnectKey)
    setDisconnectSecondsLeft(match.disconnect_seconds_left || 0)
  }
  // Clear the local countdown when the disconnect is resolved.
  if (match?.status === 'in_progress' && !match.disconnect_user_id && disconnectSecondsLeft > 0) {
    setDisconnectSecondsLeft(0)
    setPrevDisconnectKey('')
  }

  useEffect(() => {
    if (!match || match.status !== 'in_progress' || !match.disconnect_user_id) return

    const initialSeconds = match.disconnect_seconds_left || 0
    if (initialSeconds <= 0) return

    const startedAt = Date.now()
    const timer = setInterval(() => {
      const elapsed = Math.floor((Date.now() - startedAt) / 1000)
      setDisconnectSecondsLeft(Math.max(0, initialSeconds - elapsed))
    }, 1000)

    return () => clearInterval(timer)
  }, [match?.disconnect_user_id, match?.disconnect_seconds_left, match?.status, match])

  // Move mutation
  const moveMutation = useMutation({
    mutationFn: ({ matchId: id, action }: { matchId: string; action: Record<string, unknown> }) =>
      p2pApi.submitMove(id, action),
    onSuccess: (updated, variables) => {
      // The authoritative snapshot now contains the move; dropping the overlay
      // in the same batch avoids a visible snap-back.
      applyMatchSnapshot(updated)
      setPendingMove((current) => (current?.matchId === variables.matchId ? null : current))
    },
    onError: (err: unknown, variables) => {
      // The request may have committed even if the response was lost, so clear
      // the provisional paint and read back the server's actual state.
      setPendingMove((current) => (current?.matchId === variables.matchId ? null : current))
      if (variables.matchId !== matchId) return
      toast({
        title: t('p2p.match.moveRejected', 'Move rejected'),
        description: apiErrorMessage(err, t),
        type: 'error',
      })
      void refetch()
    },
    onSettled: (_data, _error, variables) => {
      if (moveInFlightRef.current === variables.matchId) moveInFlightRef.current = null
    },
  })

  // Ready mutation
  const readyMutation = useMutation({
    mutationFn: () => p2pApi.readyUp(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
    },
    onError: (err: unknown) => {
      toast({
        title: t('common.error', 'Error'),
        description: apiErrorMessage(err, t),
        type: 'error',
      })
    },
  })

  // Rematch mutation: the first request waits for the opponent, the second
  // acceptance starts a fresh match and sends both players into it.
  const rematchMutation = useMutation({
    mutationFn: () => p2pApi.requestRematch(matchId!),
    onSuccess: (result) => {
      if (result.status === 'started' && result.match) {
        queryClient.setQueryData<P2PMatch>(['p2p-match', result.match.id], (current) =>
          mergeMatchSnapshot(current, result.match!),
        )
        openRematch(result.match.id)
        return
      }
    },
    onError: (err: unknown) => {
      toast({
        title: t('p2p.match.rematchFailed', 'Rematch failed'),
        description: apiErrorMessage(err, t),
        type: 'error',
      })
    },
  })

  // Resign mutation
  const resignMutation = useMutation({
    mutationFn: () => p2pApi.resignMatch(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
      toast({ title: t('p2p.match.resignedToast', 'You resigned the match'), type: 'info' })
    },
    onError: (err: unknown) => {
      toast({
        title: t('p2p.match.couldNotResign', 'Could not resign the match'),
        description: apiErrorMessage(err, t),
        type: 'error',
      })
    },
  })

  // Cancel mutation
  const cancelMutation = useMutation({
    mutationFn: () => p2pApi.cancelMatch(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
      toast({
        title: t('p2p.match.cancelledToast', 'Invitation cancelled'),
        description: t('p2p.match.roomClosed', 'The room has been closed.'),
        type: 'info',
      })
    },
    onError: (err: unknown) => {
      toast({
        title: t('p2p.match.couldNotCancel', 'Could not cancel match'),
        description: apiErrorMessage(err, t),
        type: 'error',
      })
    },
  })

  // Escape closes dialogs without action.
  useEffect(() => {
    if (!resignDialogOpen && !cancelDialogOpen && !historyOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setResignDialogOpen(false)
        setCancelDialogOpen(false)
        setHistoryOpen(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [resignDialogOpen, cancelDialogOpen, historyOpen])

  // Keep the page behind the full-screen history view from scrolling.
  useEffect(() => {
    if (!historyOpen) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [historyOpen])

  if (isError) {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-3 px-4 text-center">
        <div className="rounded-full bg-destructive/10 p-3 text-destructive">
          <WifiOff className="h-6 w-6" />
        </div>
        <h1 className="text-lg font-bold">{t('p2p.match.couldNotLoad', 'Could not load this match')}</h1>
        <p className="text-sm text-muted-foreground">{apiErrorMessage(error, t)}</p>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate('/games')}>
            {t('p2p.match.backToLobby', 'Back to lobby')}
          </Button>
          <Button onClick={() => refetch()}>{t('common.retry', 'Try again')}</Button>
        </div>
      </div>
    )
  }

  if (isLoading || !match) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
        <p className="text-sm text-muted-foreground">{t('p2p.match.loadingArena', 'Loading duel arena…')}</p>
      </div>
    )
  }

  const p0 = match.players?.[0]
  const p1 = match.players?.[1]
  const myPlayer = match.players?.find((p) => p.user_id === user?.id)
  const isP0 = myPlayer?.player_index === 0
  const isP1 = myPlayer?.player_index === 1

  // One derived lifecycle state for the whole page. Transport status alone used
  // to drive the badge, so a client whose stream was down showed "Connecting…"
  // even after the server had started the match.
  const connection = deriveMatchConnection(
    {
      transport: streamStatus,
      hasState: true,
      matchStatus: match.status,
      isParticipant: !!myPlayer,
      playersPresent: match.players?.length ?? 0,
      allReady: (match.players?.length ?? 0) >= 2 && (match.players ?? []).every((p) => p.status === 'ready'),
      stale: streamStale,
    },
    t
  )

  const isMyTurn = match.status === 'in_progress' && match.current_turn_user_id === user?.id
  // Between rounds the server holds the finished board (with its winning line)
  // for a moment and clears the turn, so nobody is on the clock.
  const isRoundBreak = match.status === 'in_progress' && !match.current_turn_user_id
  // Tic-Tac-Toe identifies seats by shape; every other game keeps its symbol text.
  const isTicTacToe = match.game_slug === 'tic-tac-toe'
  const mySeat = isTicTacToe ? seatForPlayerIndex(myPlayer?.player_index) : null
  const isGameOver = ['completed', 'tied', 'forfeited', 'cancelled'].includes(match.status)
  // Presentation-only overlay. The cached `match` remains the authoritative
  // snapshot used by version checks, settlement and reconnect recovery.
  const boardMatch = activePendingMove ? applyOptimisticMove(match, activePendingMove) : match
  const boardLocked =
    !!activePendingMove || (moveMutation.isPending && moveMutation.variables?.matchId === matchId) || isGameOver
  const recentMatches = (matchHistory?.matches ?? [])
    .filter((historyMatch) => isTerminalMatchRecord(historyMatch))
    .slice(0, RECENT_HISTORY_LIMIT)
  // const damaHistory = (match.board_state?.move_history ?? []) as DamaHistoryEntry[]

  // Disconnect state derived from match (server-authoritative).
  const opponentDisconnected =
    match.status === 'in_progress' &&
    !!match.disconnect_user_id &&
    match.disconnect_user_id !== user?.id
  const iAmDisconnected =
    match.status === 'in_progress' &&
    match.disconnect_user_id === user?.id

  // Rematch state: a finished match either has the caller waiting on the
  // opponent, or the opponent waiting on the caller to accept.
  // Readiness comes from the server's per-player rows, so both clients display
  // the same thing instead of guessing from their own click.
  const opponentPlayer = match.players?.find((p) => p.user_id !== user?.id)
  const iAmReady = myPlayer?.status === 'ready'
  const opponentReady = opponentPlayer?.status === 'ready'

  const iRequestedRematch = !!match.rematch_requested_by && match.rematch_requested_by === user?.id
  const opponentRequestedRematch = !!match.rematch_requested_by && match.rematch_requested_by !== user?.id
  const canRematch = (match.players?.length ?? 0) === 2

  const handleCopyCode = async () => {
    if (!match.invite_code) return
    const success = await copyToClipboard(match.invite_code)
    if (success) {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      toast({
        title: t('p2p.match.codeCopied', 'Code copied!'),
        description: t('p2p.match.codeCopiedDesc', 'Share this 6-character code with your opponent.'),
        type: 'info',
      })
    } else {
      toast({
        title: t('p2p.match.copyFailed', 'Copy failed'),
        description: t('p2p.match.copyCodeManual', 'Please copy the code manually.'),
        type: 'error',
      })
    }
  }

  const handleShareTelegram = () => {
    if (!match?.invite_code) return
    const inviteUrl = buildInviteUrl(match.invite_code)
    const displayName = gameName(match.game_slug, t) || match.game_slug.replace('-', ' ')
    const message = buildInviteMessage(displayName, match.invite_code, inviteUrl, t)
    shareViaTelegram(inviteUrl, message)
  }

  const handleCopyLink = async () => {
    if (!match?.invite_code) return
    const inviteUrl = buildWebInviteUrl(match.invite_code)
    const success = await copyToClipboard(inviteUrl)
    if (success) {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({
        title: t('p2p.match.linkCopiedToast', 'Link copied!'),
        description: t('p2p.match.linkCopiedDesc', 'Challenge link copied to clipboard.'),
        type: 'info',
      })
    } else {
      toast({
        title: t('p2p.match.copyFailed', 'Copy failed'),
        description: t('p2p.match.copyLinkManual', 'Please copy the link manually.'),
        type: 'error',
      })
    }
  }

  const handleShareOptions = async () => {
    if (!match?.invite_code) return
    const inviteUrl = buildWebInviteUrl(match.invite_code)
    const displayName = gameName(match.game_slug, t) || match.game_slug.replace('-', ' ')
    const message = buildInviteMessage(displayName, match.invite_code, inviteUrl, t)
    const result = await shareLinkOrCopy({
      title: t('p2p.match.challengeTitle', { game: displayName, defaultValue: `Challenge: ${displayName}` }),
      text: message,
      url: inviteUrl,
    })
    if (result === 'copied') {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({
        title: t('p2p.match.linkCopiedToast', 'Link copied!'),
        description: t('p2p.match.linkCopiedDesc', 'Challenge link copied to clipboard.'),
        type: 'info',
      })
    }
  }

  const handleMoveAction = (action: Record<string, unknown>) => {
    // The ref closes the sub-frame gap between two rapid pointer events,
    // before React has re-rendered the disabled board.
    if (!matchId || moveInFlightRef.current === matchId) return
    const optimistic = buildOptimisticMove(match, user?.id, action)
    if (!optimistic) return
    moveInFlightRef.current = matchId
    setPendingMove({ matchId, move: optimistic })
    moveMutation.mutate({ matchId, action })
  }

  return (
    <div className="mx-auto max-w-4xl py-6 px-4 space-y-6">
      {/* Top Navigation & Status Bar */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border pb-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate('/games')}
          className="gap-1.5 text-xs text-muted-foreground -ml-2"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>{t('p2p.match.exitToLobby', 'Exit to Lobby')}</span>
        </Button>

        <div className="flex flex-wrap items-center justify-end gap-2">
          <SoundToggle isMuted={isMuted} onToggle={toggleMute} />

          {match.invite_code && (
            <Badge variant="outline" className="font-mono text-xs px-2.5 py-0.5 gap-1.5 bg-muted/40">
              <span>{t('p2p.match.codeLabel', { code: match.invite_code, defaultValue: `Code: ${match.invite_code}` })}</span>
              <button
                type="button"
                onClick={handleCopyCode}
                className="hover:text-primary transition-colors"
                title={t('p2p.match.copyCode', 'Copy code')}
              >
                {copied ? <Check className="w-3 h-3 text-success" /> : <Copy className="w-3 h-3" />}
              </button>
            </Badge>
          )}

          <Badge
            variant="outline"
            className="text-xs px-2.5 py-0.5 gap-1 border-border"
            data-testid="match-connection-badge"
            data-state={connection.state}
            title={connection.description}
          >
            {connection.state === 'failed' || connection.state === 'disconnected' ? (
              <span className="flex items-center gap-1 text-destructive">
                <WifiOff className="w-3 h-3" />
                {connection.label}
              </span>
            ) : connection.state === 'reconnecting' || connection.state === 'connecting' ? (
              <span className="flex items-center gap-1 text-warning">
                <Loader2 className="w-3 h-3 animate-spin" />
                {connection.label}
              </span>
            ) : connection.state === 'completed' ? (
              <span className="flex items-center gap-1 text-muted-foreground">
                <Flag className="w-3 h-3" />
                {connection.label}
              </span>
            ) : (
              <span className="flex items-center gap-1 text-success">
                <Wifi className="w-3 h-3" />
                {connection.label}
              </span>
            )}
          </Badge>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setHistoryOpen(true)}
            aria-haspopup="dialog"
            className="h-8 gap-1.5 border-border/80 bg-card/60 text-xs font-semibold"
          >
            <History className="h-3.5 w-3.5" />
            <span>{t('p2p.match.matchHistory', 'Match History')}</span>
          </Button>
        </div>
      </div>

      {/* Live stream lost mid-match: the board keeps syncing from the server. */}
      {connection.state === 'reconnecting' && (
        <div
          role="status"
          className="flex items-center justify-between gap-3 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs font-semibold text-warning"
        >
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
            <span>{t('p2p.match.reconnectingBanner', 'Reconnecting to the live match… your moves are safe on the server.')}</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs shrink-0 border-warning/40 text-warning hover:bg-warning/10"
            onClick={() => {
              retryStream()
              refetch()
            }}
          >
            {t('p2p.match.retryNow', 'Retry now')}
          </Button>
        </div>
      )}

      {/* The stream gave up. Keep polling the authoritative state and offer a
          real retry instead of forcing a page reload (which lost the match UI). */}
      {connection.state === 'failed' && !isGameOver && (
        <div
          role="alert"
          className="flex items-center justify-between gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs font-semibold text-destructive"
        >
          <div className="flex items-center gap-2">
            <WifiOff className="h-4 w-4 shrink-0" />
            <span>
              {t(
                'p2p.match.connectionLostBanner',
                'Live connection lost. Your match is still active on the server and is being reloaded from it automatically.'
              )}
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs shrink-0 border-destructive/40 text-destructive hover:bg-destructive/10"
            onClick={() => {
              retryStream()
              refetch()
            }}
          >
            {t('p2p.match.retryConnection', 'Retry connection')}
          </Button>
        </div>
      )}

      {/* Opponent disconnected — countdown banner shown to the connected player */}
      {opponentDisconnected && (
        <div
          role="alert"
          aria-live="polite"
          className="flex items-center justify-between gap-3 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2.5 text-xs font-semibold text-warning"
        >
          <div className="flex items-center gap-2">
            <WifiOff className="h-4 w-4 shrink-0 text-warning" />
            <span>
              {disconnectSecondsLeft > 0
                ? t('p2p.match.opponentDisconnectedWithTime', {
                    seconds: disconnectSecondsLeft,
                    defaultValue: `Your opponent disconnected. They have ${disconnectSecondsLeft}s to reconnect.`,
                  })
                : `${t('p2p.opponentDisconnectedBanner', 'Your opponent disconnected.')} ${t(
                    'p2p.match.waitingForReconnect',
                    'Waiting for them to reconnect…'
                  )}`}
            </span>
          </div>
          {disconnectSecondsLeft > 0 && (
            <span className="shrink-0 tabular-nums font-black text-warning text-sm">
              {disconnectSecondsLeft}s
            </span>
          )}
        </div>
      )}

      {/* Own reconnecting progress — shown to the player who lost the connection */}
      {iAmDisconnected && (
        <div
          role="status"
          aria-live="polite"
          className="flex items-center justify-between gap-3 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2.5 text-xs font-semibold text-warning"
        >
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
            <span>
              {disconnectSecondsLeft > 0
                ? t('p2p.match.selfDisconnectedWithTime', {
                    seconds: disconnectSecondsLeft,
                    defaultValue: `You are reconnecting. Resume within ${disconnectSecondsLeft}s or the match will be forfeited.`,
                  })
                : t('p2p.match.forfeitingSoon', 'Forfeiting soon…')}
            </span>
          </div>
          {disconnectSecondsLeft > 0 && (
            <span className="shrink-0 tabular-nums font-black text-warning text-sm">
              {disconnectSecondsLeft}s
            </span>
          )}
        </div>
      )}

      {/* Paid match summary: kept outside the board so the game stays clean. */}
      {isPaidMatch && (
        <Card className="border-[#fbbf24]/30 bg-[#fbbf24]/[0.04] shadow-sm">
          <CardContent className="flex flex-wrap items-center justify-between gap-4 p-4">
            <div className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#fbbf24]/10 text-[#b8860b] dark:text-[#fbbf24]">
                <Coins className="h-4 w-4" />
              </div>
              <div>
                <p className="text-[11px] font-bold uppercase tracking-[0.14em] text-[#b8860b] dark:text-[#fbbf24]">
                  {t('p2p.stakeSetup.title', 'Compete to Earn')}
                </p>
                <p className="text-xs text-muted-foreground">
                  {match.status === 'waiting' || match.status === 'ready'
                    ? t('p2p.match.stakesReserved', 'Stakes are reserved until the match ends')
                    : match.status === 'tied'
                      ? t('p2p.match.drawRefunded', 'Draw — both stakes refunded')
                      : match.status === 'cancelled'
                        ? t('p2p.match.cancelledRefunded', 'Cancelled — stakes refunded')
                        : t('p2p.match.matchSettled', 'Match settled')}
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs">
              <PaidMetric label={t('p2p.stakeSetup.yourStakeRow', 'Your stake')} value={formatCurrency(match.stake_amount)} />
              <PaidMetric label={t('p2p.match.opponentStake', 'Opponent stake')} value={formatCurrency(match.stake_amount)} />
              <PaidMetric label={t('p2p.prizePool', 'Prize pool')} value={formatCurrency(match.prize_pool)} />
              <PaidMetric
                label={t('p2p.match.feeLabel', {
                  fee: match.service_fee_percent.toFixed(2),
                  defaultValue: `Fee ${match.service_fee_percent.toFixed(2)}%`,
                })}
                value={formatCurrency(match.status === 'tied' || match.status === 'cancelled' ? 0 : match.total_fee || (match.prize_pool * match.service_fee_bp) / 10000)}
              />
              <PaidMetric
                label={t('p2p.match.winnerPayout', 'Winner payout')}
                value={formatCurrency(match.potential_payout)}
                highlight
              />
              {wallet && (
                <PaidMetric
                  label={t('p2p.match.available', 'Available')}
                  value={formatCurrency(wallet.available_balance ?? wallet.balance)}
                />
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Opponents Duel Header Card */}
      <Card className="border-border/80 shadow-md bg-card/60 backdrop-blur overflow-hidden">
        <CardContent className="p-4 sm:p-6">
          <div className="grid grid-cols-3 items-center">
            {/* Player 0 (Left) */}
            <div className="flex flex-col items-center sm:items-start text-center sm:text-left space-y-1">
              <div className="relative">
                <div className="w-12 h-12 sm:w-14 sm:h-14 rounded-2xl bg-info/10 border-2 border-info/40 flex items-center justify-center overflow-hidden font-black text-xl text-info">
                  {p0?.avatar_url ? (
                    <img src={p0.avatar_url} alt="" className="h-full w-full object-cover" />
                  ) : isTicTacToe && p0 ? (
                    <PlayerOneMarker variant="plain" className="h-6 w-6 sm:h-7 sm:w-7" />
                  ) : match.game_slug === 'connect-four' && p0 ? (
                    <ConnectFourToken seat={1} className="h-7 w-7 sm:h-8 sm:w-8" />
                  ) : match.game_slug === 'ethiopian-dama' && p0 ? (
                    <DamaPiece side={0} className="h-7 w-7 sm:h-8 sm:w-8" />
                  ) : (
                    p0?.symbol || 'P1'
                  )}
                </div>
                {p0?.user_id === user?.id && (
                  <span className="absolute -top-1.5 -right-1.5 bg-primary text-primary-foreground text-[10px] font-bold px-1.5 py-0.2 rounded-full">
                    {t('p2p.match.youTag', 'YOU')}
                  </span>
                )}
              </div>
              <span className="font-bold text-sm truncate max-w-[120px] sm:max-w-[160px]">
                {p0?.display_name || (isP0 ? t('p2p.match.you', 'You') : t('p2p.match.hostPlayer', 'Host Player'))}
              </span>
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span>{t('p2p.match.score', 'Score:')}</span>
                <span className="font-bold text-foreground text-sm">{match.scores?.['0'] ?? 0}</span>
              </div>
            </div>

            {/* Duel Status / Timer (Center) */}
            <div className="flex flex-col items-center justify-center text-center space-y-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {gameName(match.game_slug, t) || match.game_slug.replace('-', ' ')}
              </span>

              {match.status === 'in_progress' ? (
                <div className="space-y-1">
                  {isRoundBreak ? (
                    <>
                      <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold border border-success/30 bg-success/15 text-success">
                        <Trophy className="w-3.5 h-3.5" />
                        <span>{t('p2p.match.roundWon', 'ROUND WON')}</span>
                      </div>
                      <div className="text-[11px] font-semibold text-muted-foreground animate-pulse">
                        {t('p2p.match.nextRoundStarting', 'Next round starting…')}
                      </div>
                    </>
                  ) : (
                    <>
                      <div
                        className={[
                          'inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold transition-all',
                          isMyTurn
                            ? 'border border-success/30 bg-success/15 text-success animate-pulse'
                            : 'border border-warning/30 bg-warning/15 text-warning',
                        ].join(' ')}
                      >
                        {mySeat && <TicTacToeMarker seat={mySeat} variant="plain" className="h-3.5 w-3.5" />}
                        <Clock className="w-3.5 h-3.5" />
                        <span>{isMyTurn ? t('p2p.yourTurn', 'YOUR TURN') : t('p2p.opponentTurn', 'OPPONENT TURN')}</span>
                      </div>
                      <div className="text-2xl font-black tabular-nums tracking-tight">
                        {secondsLeft}s
                      </div>
                    </>
                  )}
                  <div className="text-[11px] text-muted-foreground">
                    {t('p2p.roundIndicator', {
                      current: match.current_round,
                      total: match.max_rounds,
                      defaultValue: `Round ${match.current_round} of ${match.max_rounds}`,
                    })}
                  </div>
                </div>
              ) : (
                <div className="py-2 flex flex-col items-center gap-1">
                  <Badge
                    variant={outcome?.type === 'win' ? 'default' : outcome?.type === 'draw' ? 'outline' : 'secondary'}
                    className={[
                      'text-xs font-black px-3 py-1 uppercase tracking-wide',
                      outcome?.type === 'win'
                        ? 'border border-success/40 bg-success/15 text-success shadow-sm shadow-success/10'
                        : outcome?.type === 'loss'
                          ? 'border border-destructive/40 bg-destructive/15 text-destructive shadow-sm shadow-destructive/10'
                          : outcome?.type === 'draw'
                            ? 'border border-warning/40 bg-warning/15 text-warning'
                            : '',
                    ].join(' ')}
                  >
                    {outcome ? outcome.title : match.status.replace('_', ' ')}
                  </Badge>
                  {outcome && (
                    <span className="text-[10px] font-bold text-muted-foreground line-clamp-1 max-w-[130px]">
                      {outcome.subtitle}
                    </span>
                  )}
                </div>
              )}
            </div>

            {/* Player 1 (Right) */}
            <div className="flex flex-col items-center sm:items-end text-center sm:text-right space-y-1">
              <div className="relative">
                <div className="w-12 h-12 sm:w-14 sm:h-14 rounded-2xl bg-warning/10 border-2 border-warning/40 flex items-center justify-center overflow-hidden font-black text-xl text-warning">
                  {p1?.avatar_url ? (
                    <img src={p1.avatar_url} alt="" className="h-full w-full object-cover" />
                  ) : isTicTacToe && p1 ? (
                    <PlayerTwoMarker variant="plain" className="h-6 w-6 sm:h-7 sm:w-7" />
                  ) : match.game_slug === 'connect-four' && p1 ? (
                    <ConnectFourToken seat={2} className="h-7 w-7 sm:h-8 sm:w-8" />
                  ) : match.game_slug === 'ethiopian-dama' && p1 ? (
                    <DamaPiece side={1} className="h-7 w-7 sm:h-8 sm:w-8" />
                  ) : (
                    p1?.symbol || (p1 ? 'P2' : '?')
                  )}
                </div>
                {p1?.user_id === user?.id && (
                  <span className="absolute -top-1.5 -left-1.5 bg-primary text-primary-foreground text-[10px] font-bold px-1.5 py-0.2 rounded-full">
                    {t('p2p.match.youTag', 'YOU')}
                  </span>
                )}
              </div>
              <span className="font-bold text-sm truncate max-w-[120px] sm:max-w-[160px]">
                {p1?.display_name || (isP1 ? t('p2p.match.you', 'You') : p1 ? t('p2p.match.challenger', 'Challenger') : t('p2p.match.waiting', 'Waiting…'))}
              </span>
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span>{t('p2p.match.score', 'Score:')}</span>
                <span className="font-bold text-foreground text-sm">{match.scores?.['1'] ?? 0}</span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Main Interactive Stage */}
      <Card className="border-border/80 shadow-xl min-h-[420px] flex flex-col justify-center">
        <CardContent className="p-4 sm:p-8 flex flex-col items-center justify-center">
          {/* State 0: MATCH CANCELLED */}
          {match.status === 'cancelled' && (
            <div className="text-center max-w-sm space-y-4 py-8 animate-in fade-in">
              <div className="w-16 h-16 rounded-full bg-muted border-2 border-border flex items-center justify-center mx-auto text-muted-foreground">
                <Users className="w-8 h-8 opacity-60" />
              </div>
              <div>
                <h3 className="text-xl font-bold">{t('p2p.match.cancelledTitle', 'Match Cancelled')}</h3>
                <p className="text-xs text-muted-foreground mt-1">
                  {t('p2p.match.cancelledDesc', 'This room was cancelled by the host or expired after 5 minutes of inactivity.')}
                </p>
              </div>
              <div className="pt-2">
                <Button onClick={() => navigate('/games')} className="font-semibold text-xs">
                  {t('p2p.match.returnToLobby', 'Return to Games Lobby')}
                </Button>
              </div>
            </div>
          )}

          {/* State 1: WAITING FOR OPPONENT */}
          {match.status === 'waiting' && (
            <div className="w-full max-w-md space-y-5 py-4 animate-in fade-in text-center">
              {/* Radar Status Animation */}
              <div className="relative inline-flex items-center justify-center">
                <div className="w-16 h-16 rounded-2xl bg-primary/10 border-2 border-primary/30 flex items-center justify-center text-primary">
                  <Users className="w-8 h-8" />
                </div>
                <span className="absolute -top-1 -right-1 flex h-4 w-4">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75" />
                  <span className="relative inline-flex rounded-full h-4 w-4 bg-success border-2 border-background" />
                </span>
              </div>

              <div>
                <h3 className="text-xl font-bold">
                  {isPaidMatch
                    ? t('p2p.match.searchingStakedOpponent', 'Searching for a Staked Opponent')
                    : t('p2p.match.waitingForChallenger', 'Waiting for Challenger')}
                </h3>
                <p className="text-xs text-muted-foreground mt-1 max-w-xs mx-auto">
                  {isPaidMatch
                    ? t('p2p.match.stakedWaitingDesc', {
                        amount: formatCurrency(match.stake_amount),
                        defaultValue: `Only a player staking exactly ${formatCurrency(match.stake_amount)} can join. Your stake is reserved while you wait.`,
                      })
                    : t(
                        'p2p.match.freeWaitingDesc',
                        'Share this invitation with a friend through Telegram or copy the room link to play!'
                      )}
                </p>
              </div>

              {/* Expiration Timer Badge */}
              <div className="flex items-center justify-center">
                <Badge variant="outline" className="font-mono text-xs px-3 py-1 gap-1.5 border-border">
                  <Clock className="w-3.5 h-3.5 text-warning" />
                  <span>
                    {waitingSecondsLeft > 0
                      ? t('p2p.match.expiresIn', {
                          time: `${Math.floor(waitingSecondsLeft / 60)}:${waitingSecondsLeft % 60 < 10 ? '0' : ''}${waitingSecondsLeft % 60}`,
                          defaultValue: `Expires in ${Math.floor(waitingSecondsLeft / 60)}:${waitingSecondsLeft % 60 < 10 ? '0' : ''}${waitingSecondsLeft % 60}`,
                        })
                      : t('p2p.match.expired', 'Expired')}
                  </span>
                </Badge>
              </div>

              {/* Room Code Card */}
              {match.invite_code && (
                <div className="p-4 bg-muted/40 rounded-2xl border border-border/80 space-y-2">
                  <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('p2p.match.privateRoomCode', 'Private Room Code')}
                  </span>
                  <div className="flex items-center justify-center gap-3">
                    <span className="font-mono text-3xl font-black tracking-widest text-primary">
                      {match.invite_code}
                    </span>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={handleCopyCode}
                      className="h-9 px-3 gap-1.5 font-semibold text-xs"
                    >
                      {copied ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-success" />
                          <span>{t('common.copied', 'Copied')}</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>{t('common.copy', 'Copy')}</span>
                        </>
                      )}
                    </Button>
                  </div>
                </div>
              )}

              {/* Sharing Actions */}
              {match.invite_code && (
                <div className="space-y-2.5">
                  <Button
                    onClick={handleShareTelegram}
                    className="w-full h-11 font-bold text-sm gap-2 bg-gradient-to-r from-sky-500 to-blue-600 hover:from-sky-600 hover:to-blue-700 text-white shadow-md shadow-sky-500/20"
                  >
                    <Send className="w-4 h-4" />
                    <span>{t('p2p.match.inviteViaTelegram', 'Invite via Telegram')}</span>
                  </Button>

                  <div className="grid grid-cols-2 gap-2">
                    <Button
                      variant="outline"
                      onClick={handleCopyLink}
                      className="h-9 text-xs font-semibold gap-1.5"
                    >
                      {copiedLink ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-success" />
                          <span>{t('p2p.match.linkCopied', 'Link Copied')}</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>{t('p2p.match.copyLink', 'Copy Link')}</span>
                        </>
                      )}
                    </Button>

                    <Button
                      variant="outline"
                      onClick={handleShareOptions}
                      className="h-9 text-xs font-semibold gap-1.5"
                    >
                      <Share2 className="w-3.5 h-3.5" />
                      <span>{t('p2p.match.shareOptions', 'Share Options')}</span>
                    </Button>
                  </div>
                </div>
              )}

              {/* Cancel Invitation Action */}
              {match.created_by === user?.id && (
                <div className="pt-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setCancelDialogOpen(true)}
                    disabled={cancelMutation.isPending}
                    className="text-xs text-muted-foreground hover:text-destructive"
                  >
                    {t('p2p.match.cancelInvitation', 'Cancel Invitation')}
                  </Button>
                </div>
              )}
            </div>
          )}

          {/* State 2: READY UP */}
          {match.status === 'ready' && (
            <div className="text-center max-w-sm space-y-4 py-8 animate-in fade-in">
              <div className="w-16 h-16 rounded-full bg-success/10 border-2 border-success/40 flex items-center justify-center mx-auto text-success">
                <Swords className="w-8 h-8" />
              </div>
              <div>
                <h3 className="text-xl font-bold">
                  {isPaidMatch ? t('p2p.match.confirmStake', 'Confirm Your Stake') : t('p2p.match.bothPlayersJoined', 'Both Players Joined!')}
                </h3>
                <p className="text-xs text-muted-foreground mt-1">
                  {isPaidMatch
                    ? t('p2p.match.confirmStakeDesc', {
                        amount: formatCurrency(match.stake_amount),
                        defaultValue: `Both players must confirm the ${formatCurrency(match.stake_amount)} stake before the match starts. Funds settle only when the match ends.`,
                      })
                    : t('p2p.match.readyUpDesc', 'Click Ready Up when you are prepared to begin the duel.')}
                </p>
              </div>

              {iAmReady ? (
                <div className="inline-flex items-center gap-2 text-sm font-semibold text-success bg-success/10 px-4 py-2 rounded-full border border-success/20">
                  <span>
                    {opponentReady
                      ? t('p2p.match.bothReadyStarting', 'Both players are ready — starting the match…')
                      : t('p2p.match.youReadyWaiting', 'You are ready! Waiting for opponent…')}
                  </span>
                </div>
              ) : (
                <div className="space-y-2">
                  <Button
                    size="lg"
                    disabled={readyMutation.isPending}
                    onClick={() => readyMutation.mutate()}
                    className="w-full font-bold shadow-lg shadow-primary/20 text-base"
                  >
                    {readyMutation.isPending
                      ? t('p2p.match.confirming', 'Confirming…')
                      : isPaidMatch
                        ? t('p2p.match.confirmStakeButton', {
                            amount: formatCurrency(match.stake_amount),
                            defaultValue: `Confirm ${formatCurrency(match.stake_amount)} Stake`,
                          })
                        : t('p2p.match.iAmReady', 'I am Ready! ⚔️')}
                  </Button>
                  <p className="text-[11px] text-muted-foreground">
                    {opponentReady
                      ? t('p2p.match.opponentIsReady', 'Your opponent is ready.')
                      : t('p2p.match.opponentNotReady', 'Your opponent has not readied up yet.')}
                  </p>
                </div>
              )}
            </div>
          )}

          {/* State 3 & 4: BOARD STAGE (Active Play & Decisive Game-Ending Review) */}
          {(match.status === 'in_progress' || isGameOver) && (
            <div
              className={`relative w-full flex flex-col items-center transition-all duration-200 ${
                isReviewingBoard ? 'pb-28 sm:pb-8' : ''
              }`}
            >
              {/* Victory Celebration Confetti Particles */}
              {isEffectsActive && outcome?.type === 'win' && <VictoryConfetti />}

              {/* Decisive Event Banner with Skip button */}
              {isGameOver && outcome && (
                <DecisiveBanner
                  outcome={outcome}
                  phase={outcomePhase}
                  onSkip={skipToAnnouncement}
                />
              )}

              {/* The game board stays mounted and highlights decisive events */}
              {match.game_slug === 'tic-tac-toe' && (
                <TicTacToeBoard
                  boardState={boardMatch.board_state}
                  isMyTurn={isMyTurn}
                  mySymbol={myPlayer?.symbol}
                  onMove={(cell) => handleMoveAction({ cell })}
                  disabled={boardLocked}
                  isGameOver={isGameOver}
                  isTie={outcome?.type === 'draw'}
                />
              )}

              {match.game_slug === 'connect-four' && (
                <ConnectFourBoard
                  boardState={boardMatch.board_state}
                  isMyTurn={isMyTurn}
                  mySymbol={myPlayer?.symbol}
                  onMove={(column) => handleMoveAction({ column })}
                  disabled={boardLocked}
                  isGameOver={isGameOver}
                  isTie={outcome?.type === 'draw'}
                />
              )}

              {match.game_slug === 'rock-paper-scissors' && (
                <RockPaperScissorsBoard
                  boardState={boardMatch.board_state}
                  isMyTurn={isMyTurn}
                  myPlayerIndex={myPlayer?.player_index}
                  scores={match.scores}
                  currentRound={match.current_round}
                  maxRounds={match.max_rounds}
                  onMove={(choice) => handleMoveAction({ choice })}
                  disabled={boardLocked}
                  isGameOver={isGameOver}
                />
              )}

              {match.game_slug === 'ethiopian-dama' && (
                <div className="flex w-full flex-col items-center gap-4">
                  <DamaBoard
                    boardState={boardMatch.board_state as DamaBoardState}
                    isMyTurn={isMyTurn}
                    myPlayerIndex={myPlayer?.player_index}
                    onMove={handleMoveAction}
                    disabled={boardLocked}
                    isGameOver={isGameOver}
                    outcome={outcome}
                  />
                  {/* <div className="w-full max-w-[560px]">
                    <DamaMoveHistory history={damaHistory} />
                  </div> */}
                </div>
              )}

              {/* Phase 3 Outcome Announcement Overlay */}
              {isGameOver && outcome && isAnnounced && (
                <GameOutcomeOverlay
                  outcome={outcome}
                  isReviewingBoard={isReviewingBoard}
                  onToggleReviewBoard={toggleReviewBoard}
                  onBackToLobby={() => navigate('/games')}
                  canRematch={canRematch}
                  onRematch={() => rematchMutation.mutate()}
                  isRematchPending={rematchMutation.isPending}
                  iRequestedRematch={iRequestedRematch}
                  opponentRequestedRematch={opponentRequestedRematch}
                />
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Surrender / Resign Button during Active Play */}
      {match.status === 'in_progress' && (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setResignDialogOpen(true)}
            disabled={resignMutation.isPending}
            className="text-muted-foreground hover:text-destructive text-xs gap-1.5"
          >
            <Flag className="w-3.5 h-3.5" />
            <span>{t('p2p.match.surrenderMatch', 'Surrender Match')}</span>
          </Button>
        </div>
      )}

      {historyOpen && (
        <MatchHistoryModal
          matches={recentMatches}
          isLoading={historyLoading}
          isError={historyError}
          onRetry={() => void refetchHistory()}
          onClose={() => setHistoryOpen(false)}
          currentUserId={user?.id}
        />
      )}

      {resignDialogOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          aria-labelledby="resign-dialog-title"
          onClick={() => setResignDialogOpen(false)}
        >
          <Card
            className="w-full max-w-sm border-border/80 shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          >
            <CardContent className="space-y-4 p-5">
              <div>
                <h2 id="resign-dialog-title" className="text-lg font-bold">
                  {t('p2p.match.resignTitle', 'Resign this match?')}
                </h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t('p2p.match.resignDesc', 'Your opponent will be awarded the result. This cannot be undone.')}
                </p>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setResignDialogOpen(false)} autoFocus>
                  {t('p2p.match.keepPlaying', 'Keep playing')}
                </Button>
                <Button
                  variant="destructive"
                  disabled={resignMutation.isPending}
                  onClick={() => {
                    setResignDialogOpen(false)
                    resignMutation.mutate()
                  }}
                >
                  {resignMutation.isPending ? t('p2p.match.resigning', 'Resigning…') : t('p2p.match.resignConfirm', 'Resign')}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {cancelDialogOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          aria-labelledby="cancel-dialog-title"
          onClick={() => setCancelDialogOpen(false)}
        >
          <Card
            className="w-full max-w-sm border-border/80 shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          >
            <CardContent className="space-y-4 p-5">
              <div className="flex items-start gap-3">
                <div className="w-10 h-10 rounded-full bg-destructive/10 text-destructive flex items-center justify-center shrink-0 mt-0.5">
                  <AlertTriangle className="h-5 w-5" />
                </div>
                <div>
                  <h2 id="cancel-dialog-title" className="text-base font-bold">
                    {t('p2p.match.cancelDialogTitle', 'Cancel Invitation?')}
                  </h2>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t(
                      'p2p.match.cancelDialogDesc',
                      'This will close the room and invalidate the invite code. Your friend will no longer be able to join.'
                    )}
                  </p>
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" size="sm" onClick={() => setCancelDialogOpen(false)} autoFocus>
                  {t('p2p.match.keepWaiting', 'Keep Waiting')}
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={cancelMutation.isPending}
                  onClick={() => {
                    setCancelDialogOpen(false)
                    cancelMutation.mutate()
                  }}
                >
                  {cancelMutation.isPending ? t('p2p.match.cancelling', 'Cancelling…') : t('p2p.match.cancelConfirm', 'Cancel Invitation')}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}

export default Component
