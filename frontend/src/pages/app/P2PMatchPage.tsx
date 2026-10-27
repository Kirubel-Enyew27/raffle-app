import { useState, useEffect, useCallback } from 'react'
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
  Sparkles,
  Loader2,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { useP2PStream, type P2PEvent } from '@/features/p2p/useP2PStream'
import { TicTacToeBoard } from '@/features/p2p/TicTacToeBoard'
import { PlayerOneMarker, PlayerTwoMarker, TicTacToeMarker } from '@/features/p2p/TicTacToeMarkers'
import { ConnectFourToken } from '@/features/p2p/ConnectFourToken'
import { DamaPiece } from '@/features/p2p/DamaPiece'
import { seatForPlayerIndex } from '@/features/p2p/ticTacToe'
import { ConnectFourBoard } from '@/features/p2p/ConnectFourBoard'
import { RockPaperScissorsBoard } from '@/features/p2p/RockPaperScissorsBoard'
import { DamaBoard, DamaMoveHistory } from '@/features/p2p/DamaBoard'
import { type DamaBoardState, type DamaHistoryEntry } from '@/features/p2p/dama'
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

  const [copied, setCopied] = useState(false)
  const [secondsLeft, setSecondsLeft] = useState<number>(0)
  const [resignDialogOpen, setResignDialogOpen] = useState(false)

  // Fetch initial match state
  const {
    data: match,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery<P2PMatch>({
    queryKey: ['p2p-match', matchId],
    queryFn: () => p2pApi.getMatchState(matchId!),
    enabled: !!matchId,
    // No polling: the match channel pushes player_joined / match_started, and
    // onReconnect refetches after a dropped stream.
  })

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
      if (event.type === 'p2p.move_made' && event.payload?.state) {
        queryClient.setQueryData(['p2p-match', matchId], event.payload.state)
      } else if (event.type === 'p2p.match_started' || event.type === 'p2p.match_completed') {
        queryClient.setQueryData(['p2p-match', matchId], event.payload)
      } else if (event.type === 'p2p.rematch_requested') {
        // The requester is told separately; only the opponent needs a nudge.
        if (event.payload?.user_id !== user?.id) {
          toast({
            title: 'Rematch requested',
            description: 'Your opponent wants to play again.',
            type: 'info',
          })
        }
        refetch()
      } else if (event.type === 'p2p.rematch_created' && event.payload?.next_match_id) {
        toast({ title: 'Rematch started', description: 'Both players agreed — good luck!', type: 'success' })
        navigate(`/games/match/${event.payload.next_match_id}`)
      } else {
        refetch()
      }
    },
    [matchId, queryClient, refetch, toast, user?.id, navigate]
  )

  const { isLive, status: streamStatus } = useP2PStream({
    matchId,
    token,
    enabled: !!matchId && !!token,
    onEvent: handleEvent,
    onReconnect: () => refetch(),
  })

  // Turn timer countdown locally
  useEffect(() => {
    if (!match || match.status !== 'in_progress') {
      setSecondsLeft(0)
      return
    }

    setSecondsLeft(match.turn_seconds_left || 0)

    const timer = setInterval(() => {
      setSecondsLeft((prev) => (prev > 0 ? prev - 1 : 0))
    }, 1000)

    return () => clearInterval(timer)
  }, [match?.current_turn_user_id, match?.turn_seconds_left, match?.status])

  // Move mutation
  const moveMutation = useMutation({
    mutationFn: (action: Record<string, unknown>) => p2pApi.submitMove(matchId!, action),
    onSuccess: (updated) => {
      queryClient.setQueryData(['p2p-match', matchId], updated)
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
      queryClient.setQueryData(['p2p-match', matchId], updated)
      toast({ title: 'Ready!', description: 'Waiting for opponent to ready up...', type: 'success' })
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
        queryClient.setQueryData(['p2p-match', result.match.id], result.match)
        toast({ title: 'Rematch started', description: 'Both players agreed — good luck!', type: 'success' })
        navigate(`/games/match/${result.match.id}`)
        return
      }
      if (result.match) {
        queryClient.setQueryData(['p2p-match', matchId], result.match)
      }
      toast({
        title: 'Rematch requested',
        description: 'Waiting for your opponent to accept…',
        type: 'info',
      })
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
      queryClient.setQueryData(['p2p-match', matchId], updated)
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

  // Escape closes the resignation dialog without resigning.
  useEffect(() => {
    if (!resignDialogOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setResignDialogOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [resignDialogOpen])

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

  const isMyTurn = match.status === 'in_progress' && match.current_turn_user_id === user?.id
  // Tic-Tac-Toe identifies seats by shape; every other game keeps its symbol text.
  const isTicTacToe = match.game_slug === 'tic-tac-toe'
  const mySeat = isTicTacToe ? seatForPlayerIndex(myPlayer?.player_index) : null
  const isGameOver = ['completed', 'tied', 'forfeited', 'cancelled'].includes(match.status)
  const damaHistory = (match.board_state?.move_history ?? []) as DamaHistoryEntry[]

  // Rematch state: a finished match either has the caller waiting on the
  // opponent, or the opponent waiting on the caller to accept.
  const iRequestedRematch = !!match.rematch_requested_by && match.rematch_requested_by === user?.id
  const opponentRequestedRematch = !!match.rematch_requested_by && match.rematch_requested_by !== user?.id
  const canRematch = (match.players?.length ?? 0) === 2

  const handleCopyCode = () => {
    if (!match.invite_code) return
    navigator.clipboard.writeText(match.invite_code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast({ title: 'Code copied!', description: 'Share this 6-character code with your opponent.', type: 'info' })
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

          <Badge variant="outline" className="text-xs px-2.5 py-0.5 gap-1 border-border">
            {isLive ? (
              <span className="flex items-center gap-1 text-emerald-500">
                <Wifi className="w-3 h-3" />
                Live
              </span>
            ) : (
              <span className="flex items-center gap-1 text-amber-500">
                <WifiOff className="w-3 h-3" />
                Connecting
              </span>
            )}
          </Badge>
        </div>
      </div>

      {streamStatus === 'reconnecting' && !isGameOver && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs font-semibold text-amber-700 dark:text-amber-300"
        >
          <WifiOff className="h-4 w-4 shrink-0" />
          <span>Reconnecting to the live match… your moves are safe on the server.</span>
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
      <Card className="border-border/80 shadow-xl overflow-hidden min-h-[420px] flex flex-col justify-center">
        <CardContent className="p-4 sm:p-8 flex flex-col items-center justify-center">
          {/* State 1: WAITING FOR OPPONENT */}
          {match.status === 'waiting' && (
            <div className="text-center max-w-sm space-y-4 py-8 animate-in fade-in">
              <div className="w-16 h-16 rounded-full bg-primary/10 border-2 border-primary/30 flex items-center justify-center mx-auto animate-pulse text-primary">
                <Users className="w-8 h-8" />
              </div>
              <div>
                <h3 className="text-xl font-bold">Waiting for Challenger</h3>
                <p className="text-xs text-muted-foreground mt-1">
                  Share your room code with a friend or wait for another player to join through the lobby.
                </p>
              </div>

              {match.invite_code && (
                <div className="p-3 bg-muted/50 rounded-xl border border-border/80 flex items-center justify-between gap-3">
                  <span className="font-mono text-lg font-black tracking-widest text-primary">
                    {match.invite_code}
                  </span>
                  <Button size="sm" onClick={handleCopyCode} className="h-8 gap-1.5 font-semibold text-xs">
                    {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copied ? 'Copied' : 'Copy Code'}</span>
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

              {myPlayer?.status === 'ready' ? (
                <div className="inline-flex items-center gap-2 text-sm font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-4 py-2 rounded-full border border-emerald-500/20">
                  <Sparkles className="w-4 h-4" />
                  <span>You are ready! Waiting for opponent…</span>
                </div>
              ) : (
                <Button
                  size="lg"
                  disabled={readyMutation.isPending}
                  onClick={() => readyMutation.mutate()}
                  className="w-full font-bold shadow-lg shadow-primary/20 text-base"
                >
                  {readyMutation.isPending ? 'Confirming…' : 'I am Ready! ⚔️'}
                </Button>
              )}
            </div>
          )}

          {/* State 3 & 4: BOARD STAGE (Active Play & Decisive Game-Ending Review) */}
          {(match.status === 'in_progress' || isGameOver) && (
            <div className="relative w-full flex flex-col items-center">
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
    </div>
  )
}

export default Component
