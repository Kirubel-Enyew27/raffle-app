import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Users,
  Swords,
  Clock,
  Copy,
  Check,
  Flag,
  ArrowLeft,
  Wifi,
  WifiOff,
  Loader2,
  Send,
  Share2,
  AlertTriangle,
  Trophy,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
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
import { useGameSounds } from '@/features/p2p/useGameSounds'
import { SoundToggle } from '@/features/p2p/SoundToggle'
import {
  useGameOutcomeController,
  GameOutcomeOverlay,
  DecisiveBanner,
  VictoryConfetti,
} from '@/features/p2p/outcome'
import type { P2PMatch } from '@/types/api'

// Surfaces the API's error message (axios error shape) without widening to any.
function apiErrorMessage(err: unknown): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return 'Something went wrong'
}

export function Component() {
  const { id: matchId } = useParams<{ id: string }>()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()
  const handledRematchRequestsRef = useRef(new Set<string>())
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
        toast({ title: 'Match cancelled', description: 'This invitation has been cancelled.', type: 'info' })
        refetch()
      } else {
        refetch()
      }
    },
    [matchId, applyMatchSnapshot, refetch, toast, user?.id, openRematch]
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
    mutationFn: (action: Record<string, unknown>) => p2pApi.submitMove(matchId!, action),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
    },
    onError: (err: unknown) => {
      toast({
        title: 'Move rejected',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Ready mutation
  const readyMutation = useMutation({
    mutationFn: () => p2pApi.readyUp(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
      // toast({ title: 'Ready!', description: 'Waiting for opponent to ready up...', type: 'success' })
    },
    onError: (err: unknown) => {
      toast({
        title: 'Error',
        description: apiErrorMessage(err),
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
        // toast({ title: 'Rematch started', description: 'Both players agreed — good luck!', type: 'success' })
        openRematch(result.match.id)
        return
      }
      // if (result.match) {
      //   applyMatchSnapshot(result.match)
      //   toast({
      //     title: 'Rematch requested',
      //     description: 'Your opponent will be notified. Waiting for their response…',
      //     type: 'info',
      //   })
      // }
      // toast({
      //   title: 'Rematch requested',
      //   description: 'Waiting for your opponent to accept…',
      //   type: 'info',
      // })
    },
    onError: (err: unknown) => {
      toast({
        title: 'Rematch failed',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Resign mutation
  const resignMutation = useMutation({
    mutationFn: () => p2pApi.resignMatch(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
      toast({ title: 'You resigned the match', type: 'info' })
    },
    onError: (err: unknown) => {
      toast({
        title: 'Could not resign',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Cancel mutation
  const cancelMutation = useMutation({
    mutationFn: () => p2pApi.cancelMatch(matchId!),
    onSuccess: (updated) => {
      applyMatchSnapshot(updated)
      toast({ title: 'Invitation cancelled', description: 'The room has been closed.', type: 'info' })
    },
    onError: (err: unknown) => {
      toast({
        title: 'Could not cancel match',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Escape closes dialogs without action.
  useEffect(() => {
    if (!resignDialogOpen && !cancelDialogOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setResignDialogOpen(false)
        setCancelDialogOpen(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [resignDialogOpen, cancelDialogOpen])

  if (isError) {
    return (
      <div className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-3 px-4 text-center">
        <div className="rounded-full bg-destructive/10 p-3 text-destructive">
          <WifiOff className="h-6 w-6" />
        </div>
        <h1 className="text-lg font-bold">Could not load this match</h1>
        <p className="text-sm text-muted-foreground">{apiErrorMessage(error)}</p>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate('/games')}>
            Back to lobby
          </Button>
          <Button onClick={() => refetch()}>Try again</Button>
        </div>
      </div>
    )
  }

  if (isLoading || !match) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
        <p className="text-sm text-muted-foreground">Loading duel arena…</p>
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
  const connection = deriveMatchConnection({
    transport: streamStatus,
    hasState: true,
    matchStatus: match.status,
    isParticipant: !!myPlayer,
    playersPresent: match.players?.length ?? 0,
    allReady: (match.players?.length ?? 0) >= 2 && (match.players ?? []).every((p) => p.status === 'ready'),
    // Staleness is the poll timer's observation; reading the clock here would
    // be an impure render.
    stale: streamStale,
  })

  const isMyTurn = match.status === 'in_progress' && match.current_turn_user_id === user?.id
  // Between rounds the server holds the finished board (with its winning line)
  // for a moment and clears the turn, so nobody is on the clock.
  const isRoundBreak = match.status === 'in_progress' && !match.current_turn_user_id
  // Tic-Tac-Toe identifies seats by shape; every other game keeps its symbol text.
  const isTicTacToe = match.game_slug === 'tic-tac-toe'
  const mySeat = isTicTacToe ? seatForPlayerIndex(myPlayer?.player_index) : null
  const isGameOver = ['completed', 'tied', 'forfeited', 'cancelled'].includes(match.status)
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
      toast({ title: 'Code copied!', description: 'Share this 6-character code with your opponent.', type: 'info' })
    } else {
      toast({ title: 'Copy failed', description: 'Please copy the code manually.', type: 'error' })
    }
  }

  const handleShareTelegram = () => {
    if (!match?.invite_code) return
    const inviteUrl = buildInviteUrl(match.invite_code)
    const gameName = match.game_slug.replace('-', ' ')
    const message = buildInviteMessage(gameName, match.invite_code, inviteUrl)
    shareViaTelegram(inviteUrl, message)
  }

  const handleCopyLink = async () => {
    if (!match?.invite_code) return
    const inviteUrl = buildWebInviteUrl(match.invite_code)
    const success = await copyToClipboard(inviteUrl)
    if (success) {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({ title: 'Link copied!', description: 'Challenge link copied to clipboard.', type: 'info' })
    } else {
      toast({ title: 'Copy failed', description: 'Please copy the link manually.', type: 'error' })
    }
  }

  const handleShareOptions = async () => {
    if (!match?.invite_code) return
    const inviteUrl = buildWebInviteUrl(match.invite_code)
    const gameName = match.game_slug.replace('-', ' ')
    const message = buildInviteMessage(gameName, match.invite_code, inviteUrl)
    const result = await shareLinkOrCopy({
      title: `Challenge: ${gameName}`,
      text: message,
      url: inviteUrl,
    })
    if (result === 'copied') {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({ title: 'Link copied!', description: 'Challenge link copied to clipboard.', type: 'info' })
    }
  }

  const handleMoveAction = (action: Record<string, unknown>) => {
    moveMutation.mutate(action)
  }

  return (
    <div className="mx-auto max-w-4xl py-6 px-4 space-y-6">
      {/* Top Navigation & Status Bar */}
      <div className="flex items-center justify-between border-b border-border pb-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate('/games')}
          className="gap-1.5 text-xs text-muted-foreground -ml-2"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Exit to Lobby</span>
        </Button>

        <div className="flex items-center gap-2">
          <SoundToggle isMuted={isMuted} onToggle={toggleMute} />

          {match.invite_code && (
            <Badge variant="outline" className="font-mono text-xs px-2.5 py-0.5 gap-1.5 bg-muted/40">
              <span>Code: {match.invite_code}</span>
              <button
                type="button"
                onClick={handleCopyCode}
                className="hover:text-primary transition-colors"
                title="Copy code"
              >
                {copied ? <Check className="w-3 h-3 text-emerald-500" /> : <Copy className="w-3 h-3" />}
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
              <span className="flex items-center gap-1 text-amber-500">
                <Loader2 className="w-3 h-3 animate-spin" />
                {connection.label}
              </span>
            ) : connection.state === 'completed' ? (
              <span className="flex items-center gap-1 text-muted-foreground">
                <Flag className="w-3 h-3" />
                {connection.label}
              </span>
            ) : (
              <span className="flex items-center gap-1 text-emerald-500">
                <Wifi className="w-3 h-3" />
                {connection.label}
              </span>
            )}
          </Badge>
        </div>
      </div>

      {/* Live stream lost mid-match: the board keeps syncing from the server. */}
      {connection.state === 'reconnecting' && (
        <div
          role="status"
          className="flex items-center justify-between gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs font-semibold text-amber-700 dark:text-amber-300"
        >
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
            <span>Reconnecting to the live match… your moves are safe on the server.</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs shrink-0 border-amber-500/40 text-amber-700 hover:bg-amber-500/10 dark:text-amber-300"
            onClick={() => {
              retryStream()
              refetch()
            }}
          >
            Retry now
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
              Live connection lost. Your match is still active on the server and is being
              reloaded from it automatically.
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
            Retry connection
          </Button>
        </div>
      )}

      {/* Opponent disconnected — countdown banner shown to the connected player */}
      {opponentDisconnected && (
        <div
          role="alert"
          aria-live="polite"
          className="flex items-center justify-between gap-3 rounded-lg border border-orange-500/40 bg-orange-500/10 px-3 py-2.5 text-xs font-semibold text-orange-700 dark:text-orange-300"
        >
          <div className="flex items-center gap-2">
            <WifiOff className="h-4 w-4 shrink-0 text-orange-500" />
            <span>
              Your opponent disconnected.{' '}
              {disconnectSecondsLeft > 0
                ? `They have ${disconnectSecondsLeft}s to reconnect.`
                : 'Waiting for them to reconnect…'}
            </span>
          </div>
          {disconnectSecondsLeft > 0 && (
            <span className="shrink-0 tabular-nums font-black text-orange-600 dark:text-orange-300 text-sm">
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
          className="flex items-center justify-between gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2.5 text-xs font-semibold text-amber-700 dark:text-amber-300"
        >
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
            <span>
              You are reconnecting.{' '}
              {disconnectSecondsLeft > 0
                ? `Resume within ${disconnectSecondsLeft}s or the match will be forfeited.`
                : 'Forfeiting soon…'}
            </span>
          </div>
          {disconnectSecondsLeft > 0 && (
            <span className="shrink-0 tabular-nums font-black text-amber-600 dark:text-amber-300 text-sm">
              {disconnectSecondsLeft}s
            </span>
          )}
        </div>
      )}

      {/* Opponents Duel Header Card */}
      <Card className="border-border/80 shadow-md bg-card/60 backdrop-blur overflow-hidden">
        <CardContent className="p-4 sm:p-6">
          <div className="grid grid-cols-3 items-center">
            {/* Player 0 (Left) */}
            <div className="flex flex-col items-center sm:items-start text-center sm:text-left space-y-1">
              <div className="relative">
                <div className="w-12 h-12 sm:w-14 sm:h-14 rounded-2xl bg-sky-500/10 border-2 border-sky-500/40 flex items-center justify-center overflow-hidden font-black text-xl text-sky-500">
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
                    YOU
                  </span>
                )}
              </div>
              <span className="font-bold text-sm truncate max-w-[120px] sm:max-w-[160px]">
                {p0?.display_name || (isP0 ? 'You' : 'Host Player')}
              </span>
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span>Score:</span>
                <span className="font-bold text-foreground text-sm">{match.scores?.['0'] ?? 0}</span>
              </div>
            </div>

            {/* Duel Status / Timer (Center) */}
            <div className="flex flex-col items-center justify-center text-center space-y-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {match.game_slug.replace('-', ' ')}
              </span>

              {match.status === 'in_progress' ? (
                <div className="space-y-1">
                  {isRoundBreak ? (
                    <>
                      <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30">
                        <Trophy className="w-3.5 h-3.5" />
                        <span>ROUND WON</span>
                      </div>
                      <div className="text-[11px] font-semibold text-muted-foreground animate-pulse">
                        Next round starting…
                      </div>
                    </>
                  ) : (
                    <>
                      <div
                        className={[
                          'inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold transition-all',
                          isMyTurn
                            ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 animate-pulse'
                            : 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border border-amber-500/30',
                        ].join(' ')}
                      >
                        {mySeat && <TicTacToeMarker seat={mySeat} variant="plain" className="h-3.5 w-3.5" />}
                        <Clock className="w-3.5 h-3.5" />
                        <span>{isMyTurn ? 'YOUR TURN' : 'OPPONENT TURN'}</span>
                      </div>
                      <div className="text-2xl font-black tabular-nums tracking-tight">
                        {secondsLeft}s
                      </div>
                    </>
                  )}
                  <div className="text-[11px] text-muted-foreground">
                    Round {match.current_round} of {match.max_rounds}
                  </div>
                </div>
              ) : (
                <div className="py-2 flex flex-col items-center gap-1">
                  <Badge
                    variant={outcome?.type === 'win' ? 'default' : outcome?.type === 'draw' ? 'outline' : 'secondary'}
                    className={[
                      'text-xs font-black px-3 py-1 uppercase tracking-wide',
                      outcome?.type === 'win'
                        ? 'bg-emerald-500 text-white shadow-sm shadow-emerald-500/30'
                        : outcome?.type === 'loss'
                          ? 'bg-rose-500 text-white shadow-sm shadow-rose-500/30'
                          : outcome?.type === 'draw'
                            ? 'bg-amber-500/20 text-amber-600 dark:text-amber-400 border-amber-500/40'
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
                <div className="w-12 h-12 sm:w-14 sm:h-14 rounded-2xl bg-amber-500/10 border-2 border-amber-500/40 flex items-center justify-center overflow-hidden font-black text-xl text-amber-500">
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
                    YOU
                  </span>
                )}
              </div>
              <span className="font-bold text-sm truncate max-w-[120px] sm:max-w-[160px]">
                {p1?.display_name || (isP1 ? 'You' : p1 ? 'Challenger' : 'Waiting…')}
              </span>
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span>Score:</span>
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
                <h3 className="text-xl font-bold">Match Cancelled</h3>
                <p className="text-xs text-muted-foreground mt-1">
                  This room was cancelled by the host or expired after 5 minutes of inactivity.
                </p>
              </div>
              <div className="pt-2">
                <Button onClick={() => navigate('/games')} className="font-semibold text-xs">
                  Return to Games Lobby
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
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex rounded-full h-4 w-4 bg-emerald-500 border-2 border-background" />
                </span>
              </div>

              <div>
                <h3 className="text-xl font-bold">Waiting for Challenger</h3>
                <p className="text-xs text-muted-foreground mt-1 max-w-xs mx-auto">
                  Share this invitation with a friend through Telegram or copy the room link to play!
                </p>
              </div>

              {/* Expiration Timer Badge */}
              <div className="flex items-center justify-center">
                <Badge variant="outline" className="font-mono text-xs px-3 py-1 gap-1.5 border-border">
                  <Clock className="w-3.5 h-3.5 text-amber-500" />
                  <span>
                    {waitingSecondsLeft > 0
                      ? `Expires in ${Math.floor(waitingSecondsLeft / 60)}:${waitingSecondsLeft % 60 < 10 ? '0' : ''}${waitingSecondsLeft % 60}`
                      : 'Expired'}
                  </span>
                </Badge>
              </div>

              {/* Room Code Card */}
              {match.invite_code && (
                <div className="p-4 bg-muted/40 rounded-2xl border border-border/80 space-y-2">
                  <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    Private Room Code
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
                          <Check className="w-3.5 h-3.5 text-emerald-500" />
                          <span>Copied</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy</span>
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
                    <span>Invite via Telegram</span>
                  </Button>

                  <div className="grid grid-cols-2 gap-2">
                    <Button
                      variant="outline"
                      onClick={handleCopyLink}
                      className="h-9 text-xs font-semibold gap-1.5"
                    >
                      {copiedLink ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-emerald-500" />
                          <span>Link Copied</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy Link</span>
                        </>
                      )}
                    </Button>

                    <Button
                      variant="outline"
                      onClick={handleShareOptions}
                      className="h-9 text-xs font-semibold gap-1.5"
                    >
                      <Share2 className="w-3.5 h-3.5" />
                      <span>Share Options</span>
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
                    Cancel Invitation
                  </Button>
                </div>
              )}
            </div>
          )}

          {/* State 2: READY UP */}
          {match.status === 'ready' && (
            <div className="text-center max-w-sm space-y-4 py-8 animate-in fade-in">
              <div className="w-16 h-16 rounded-full bg-emerald-500/10 border-2 border-emerald-500/40 flex items-center justify-center mx-auto text-emerald-500">
                <Swords className="w-8 h-8" />
              </div>
              <div>
                <h3 className="text-xl font-bold">Both Players Joined!</h3>
                <p className="text-xs text-muted-foreground mt-1">
                  Click Ready Up when you are prepared to begin the duel.
                </p>
              </div>

              {iAmReady ? (
                <div className="inline-flex items-center gap-2 text-sm font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-4 py-2 rounded-full border border-emerald-500/20">
                  <span>
                    {opponentReady
                      ? 'Both players are ready — starting the match…'
                      : 'You are ready! Waiting for opponent…'}
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
                    {readyMutation.isPending ? 'Confirming…' : 'I am Ready! ⚔️'}
                  </Button>
                  <p className="text-[11px] text-muted-foreground">
                    {opponentReady ? 'Your opponent is ready.' : 'Your opponent has not readied up yet.'}
                  </p>
                </div>
              )}
            </div>
          )}

          {/* State 3 & 4: BOARD STAGE (Active Play & Decisive Game-Ending Review) */}
          {(match.status === 'in_progress' || isGameOver) && (
            <div className="relative isolate w-full flex flex-col items-center">
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
                  boardState={match.board_state}
                  isMyTurn={isMyTurn}
                  mySymbol={myPlayer?.symbol}
                  onMove={(cell) => handleMoveAction({ cell })}
                  disabled={moveMutation.isPending || isGameOver}
                  isGameOver={isGameOver}
                  isTie={outcome?.type === 'draw'}
                />
              )}

              {match.game_slug === 'connect-four' && (
                <ConnectFourBoard
                  boardState={match.board_state}
                  isMyTurn={isMyTurn}
                  mySymbol={myPlayer?.symbol}
                  onMove={(column) => handleMoveAction({ column })}
                  disabled={moveMutation.isPending || isGameOver}
                  isGameOver={isGameOver}
                  isTie={outcome?.type === 'draw'}
                />
              )}

              {match.game_slug === 'rock-paper-scissors' && (
                <RockPaperScissorsBoard
                  boardState={match.board_state}
                  isMyTurn={isMyTurn}
                  myPlayerIndex={myPlayer?.player_index}
                  scores={match.scores}
                  currentRound={match.current_round}
                  maxRounds={match.max_rounds}
                  onMove={(choice) => handleMoveAction({ choice })}
                  disabled={moveMutation.isPending || isGameOver}
                  isGameOver={isGameOver}
                />
              )}

              {match.game_slug === 'ethiopian-dama' && (
                <div className="flex w-full flex-col items-center gap-4">
                  <DamaBoard
                    boardState={match.board_state as DamaBoardState}
                    isMyTurn={isMyTurn}
                    myPlayerIndex={myPlayer?.player_index}
                    onMove={handleMoveAction}
                    disabled={moveMutation.isPending || isGameOver}
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
            <span>Surrender Match</span>
          </Button>
        </div>
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
                  Resign this match?
                </h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  Your opponent will be awarded the result. This cannot be undone.
                </p>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setResignDialogOpen(false)} autoFocus>
                  Keep playing
                </Button>
                <Button
                  variant="destructive"
                  disabled={resignMutation.isPending}
                  onClick={() => {
                    setResignDialogOpen(false)
                    resignMutation.mutate()
                  }}
                >
                  {resignMutation.isPending ? 'Resigning…' : 'Resign'}
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
                    Cancel Invitation?
                  </h2>
                  <p className="mt-1 text-xs text-muted-foreground">
                    This will close the room and invalidate the invite code. Your friend will no longer be able to join.
                  </p>
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" size="sm" onClick={() => setCancelDialogOpen(false)} autoFocus>
                  Keep Waiting
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
                  {cancelMutation.isPending ? 'Cancelling…' : 'Cancel Invitation'}
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
