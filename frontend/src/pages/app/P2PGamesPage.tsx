import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Gamepad2,
  Swords,
  Users,
  PlusCircle,
  RefreshCw,
  Trophy,
  Clock,
  ArrowUpRight,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { useP2PStream } from '@/features/p2p/useP2PStream'
import { GameIcon } from '@/features/p2p/GameIcon'
import { GameArtwork } from '@/features/p2p/GameArtwork'
import { ChallengeFriendModal } from '@/features/p2p/ChallengeFriendModal'
import { AcceptInviteModal } from '@/features/p2p/AcceptInviteModal'
import { extractStartInviteCode } from '@/lib/telegram'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/hooks/usePageTitle'

function apiErrorMessage(err: unknown, fallback = 'Something went wrong'): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return fallback
}

/** One-line pitch shown on the lobby cards, matching the product artwork. */
const GAME_TAGLINES: Record<string, string> = {
  'ethiopian-dama': 'Orthogonal strategy.',
  'tic-tac-toe': 'Three in a row.',
  'rock-paper-scissors': 'Read your rival.',
  'connect-four': 'Outconnect them.',
}

/** Match format chip: Dama is one decisive game, the others are best-of-three. */
const GAME_FORMATS: Record<string, string> = {
  'ethiopian-dama': 'SINGLE GAME',
  'tic-tac-toe': 'BO3',
  'rock-paper-scissors': 'BO3',
  'connect-four': 'BO3',
}

/** Per-game accent used for the card's ambient light. */
const GAME_ACCENTS: Record<string, string> = {
  'ethiopian-dama': '#fbbf24',
  'tic-tac-toe': '#ff5b55',
  'rock-paper-scissors': '#a78bfa',
  'connect-four': '#34d399',
}

const CARD = 'rounded-2xl border border-border/80 bg-card'

export function Component() {
  usePageTitle('Games')
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const [searchParams, setSearchParams] = useSearchParams()
  const [inviteCodeInput, setInviteCodeInput] = useState('')
  const [activeTab, setActiveTab] = useState<'lobby' | 'history'>('lobby')
  const [createModalOpen, setCreateModalOpen] = useState(false)
  const [challengeModalOpen, setChallengeModalOpen] = useState(false)
  const [challengeGameSlug, setChallengeGameSlug] = useState<string | undefined>()
  const [enteredCode, setEnteredCode] = useState<string | null>(null)
  const [selectedGameSlug, setSelectedGameSlug] = useState<string>('tic-tac-toe')
  const [matchType, setMatchType] = useState<'public_queue' | 'private_room'>('public_queue')

  // Resolve active invite code from entered state, URL search params, or Telegram WebApp start param
  const urlInviteCode = (
    searchParams.get('join') ||
    searchParams.get('code') ||
    extractStartInviteCode() ||
    ''
  ).trim().toUpperCase()
  const acceptCode = enteredCode || (urlInviteCode ? urlInviteCode : null)

  const handleCloseAccept = () => {
    setEnteredCode(null)
    if (searchParams.has('join') || searchParams.has('code') || searchParams.has('tgWebAppStartParam')) {
      setSearchParams(
        (prev) => {
          const updated = new URLSearchParams(prev)
          updated.delete('join')
          updated.delete('code')
          updated.delete('tgWebAppStartParam')
          return updated
        },
        { replace: true }
      )
    }
  }

  // Fetch games
  const { data: games = [] } = useQuery({
    queryKey: ['p2p-games'],
    queryFn: p2pApi.listGames,
  })

  // Fetch lobby waiting matches. Refreshes are pushed over the shared
  // realtime channel below, so this query does not poll.
  const { data: lobbyMatches = [], isLoading: lobbyLoading, refetch: refetchLobby } = useQuery({
    queryKey: ['p2p-lobby'],
    queryFn: () => p2pApi.listLobby(),
  })

  useP2PStream({
    channel: 'p2p_lobby',
    token,
    enabled: !!token,
    onEvent: (event) => {
      if (event.type === 'p2p.lobby_updated') {
        queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      }
    },
    onReconnect: () => refetchLobby(),
  })

  // Fetch user matches
  const { data: myMatchesData, isLoading: historyLoading } = useQuery({
    queryKey: ['p2p-my-matches'],
    queryFn: () => p2pApi.listMyMatches(20, 0),
    enabled: !!user,
  })

  const myMatches = myMatchesData?.matches || []

  // Create match mutation
  const createMutation = useMutation({
    mutationFn: (payload: {
      game_slug: string
      match_type: 'public_queue' | 'private_room'
      stake_amount: number
      auto_match: boolean
    }) => p2pApi.createMatch(payload),
    onSuccess: (match) => {
      queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      queryClient.invalidateQueries({ queryKey: ['p2p-my-matches'] })
      setCreateModalOpen(false)
      navigate(`/games/match/${match.id}`)
    },
    onError: (err: unknown) => {
      toast({
        title: 'Could not create match',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Join match mutation
  const joinMutation = useMutation({
    mutationFn: (matchId: string) => p2pApi.joinMatch(matchId),
    onSuccess: (match) => {
      queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      navigate(`/games/match/${match.id}`)
    },
    onError: (err: unknown) => {
      toast({
        title: 'Failed to join match',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  const handleCreateMatch = (gameSlug: string, isPrivate = false, autoMatch = false) => {
    if (!user) {
      navigate('/login')
      return
    }
    createMutation.mutate({
      game_slug: gameSlug,
      match_type: isPrivate ? 'private_room' : 'public_queue',
      stake_amount: 0,
      auto_match: autoMatch,
    })
  }

  // Join by code preview handler
  const handleJoinByCode = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = inviteCodeInput.trim().toUpperCase()
    if (!trimmed) return
    if (trimmed.length < 4) {
      toast({
        title: 'Invalid Code Format',
        description: 'Challenge codes must be at least 4 characters long.',
        type: 'error',
      })
      return
    }
    setEnteredCode(trimmed)
    setInviteCodeInput('')
  }

  const openChallenge = (slug?: string) => {
    setChallengeGameSlug(slug)
    setChallengeModalOpen(true)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 px-4 py-6 sm:space-y-8 sm:px-6 sm:py-8">
      {/* Hero */}
      <section className="relative overflow-hidden rounded-3xl border border-[#2a1416] bg-[#101012] p-6 sm:p-10">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-24 -top-28 h-80 w-80 rounded-full bg-[#ff5b55]/20 blur-3xl"
        />
        <div className="relative z-10 max-w-2xl space-y-4">
          {/* <span className="inline-flex items-center gap-2 rounded-full border border-[#ff5b55]/30 bg-[#ff5b55]/10 px-3 py-1 text-[11px] font-bold uppercase tracking-[0.14em] text-[#ff8a80]">
            <Swords className="h-3 w-3" />
            P2P Competitive Arenas
          </span> */}

          <h1 className="text-3xl font-black tracking-tight text-white sm:text-4xl md:text-5xl">
            Play Live Against Other Players
          </h1>

          <p className="max-w-xl text-sm leading-relaxed text-[#9ca1a8] sm:text-base">
            Fast, server-authoritative duels. Jump into public matchmaking or send a private
            challenge code to a friend.
          </p>

          <div className="flex flex-col gap-2.5 pt-2 sm:flex-row sm:items-center">
            <Button
              onClick={() => openChallenge()}
              className="h-11 gap-2 px-5 text-sm shadow-lg shadow-[#ff5b55]/20"
            >
              <Swords className="h-4 w-4" />
              Challenge a Friend
            </Button>
            <Button
              variant="outline"
              onClick={() => setCreateModalOpen(true)}
              className="h-11 gap-2 border-[#2c2f36] bg-[#17181d] px-5 text-sm font-semibold text-white hover:border-[#3a3e46] hover:bg-[#1e2027] hover:text-white"
            >
              <PlusCircle className="h-4 w-4" />
              Host Custom Match
            </Button>
          </div>
        </div>
      </section>

      {/* Join with Invite Code */}
      <section className={cn(CARD, 'p-4 sm:p-5')}>
        <form onSubmit={handleJoinByCode} className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <p className="flex items-center gap-2 whitespace-nowrap text-sm font-semibold text-white">
            <Gamepad2 className="h-4 w-4 text-[#ff5b55]" />
            Have a Private Invite Code?
          </p>
          <div className="flex w-full flex-1 items-center gap-2">
            <Input
              placeholder="Enter 6-character code (e.g. ABC9X2)"
              value={inviteCodeInput}
              onChange={(e) => setInviteCodeInput(e.target.value.toUpperCase())}
              maxLength={8}
              className="h-10 font-mono uppercase tracking-[0.18em]"
            />
            <Button
              type="submit"
              disabled={!inviteCodeInput.trim()}
              className="h-10 shrink-0 px-5"
            >
              Preview & Join
            </Button>
          </div>
        </form>
      </section>

      {/* Games catalog */}
      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="flex items-center gap-2 text-xl font-bold tracking-tight text-white">
            <Gamepad2 className="h-5 w-5 text-[#ff5b55]" />
            Available Games
          </h2>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              onClick={() => openChallenge()}
              className="h-8 gap-1.5 text-xs"
            >
              <Swords className="h-3.5 w-3.5" />
              Challenge Friend
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCreateModalOpen(true)}
              className="h-8 gap-1.5 border-[#2c2f36] bg-[#17181d] text-xs text-white hover:bg-[#1e2027] hover:text-white"
            >
              <PlusCircle className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Host Custom Match</span>
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {games.map((g) => (
            <article
              key={g.slug}
              className="group relative flex flex-col overflow-hidden rounded-2xl border border-[#23252b] bg-[linear-gradient(180deg,#15161b_0%,#121317_55%,#0f1013_100%)] transition-all duration-300 hover:-translate-y-0.5 hover:border-[#ff5b55]/45 hover:shadow-[0_18px_44px_-24px_rgba(255,91,85,0.55)] active:translate-y-0 active:scale-[0.995]"
            >
              {/* Board artwork — custom per-game illustration with ambient light. */}
              <div className="relative h-32 overflow-hidden border-b border-[#1f2126] bg-[#0c0d10]">
                <div
                  aria-hidden="true"
                  className="absolute inset-0"
                  style={{
                    background: `radial-gradient(85% 95% at 50% 115%, ${GAME_ACCENTS[g.slug] ?? '#ff5b55'}26, transparent 72%)`,
                  }}
                />
                <div className="absolute inset-0 transition-transform duration-500 ease-out motion-safe:group-hover:scale-[1.04]">
                  <GameArtwork slug={g.slug} />
                </div>
                <div
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#121317] via-transparent to-transparent"
                />
                <div
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 -translate-x-full bg-gradient-to-r from-transparent via-white/5 to-transparent transition-transform duration-700 group-hover:translate-x-full"
                />
                <button
                  type="button"
                  onClick={() => handleCreateMatch(g.slug, false, true)}
                  disabled={createMutation.isPending}
                  aria-label={`Quick play ${g.title}`}
                  className="absolute right-3 top-3 rounded-lg border border-[#2c2f36] bg-[#0c0d10]/70 p-1.5 text-[#ff5b55] backdrop-blur transition-all hover:border-[#ff5b55]/50 hover:text-[#ff8a80] hover:shadow-[0_0_18px_-4px_rgba(255,91,85,0.7)]"
                >
                  <ArrowUpRight className="h-4 w-4" />
                </button>
                <span className="absolute bottom-3 left-3 rounded-full border border-[#2c2f36] bg-[#0c0d10]/70 px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.16em] text-[#9ca1a8] backdrop-blur">
                  {GAME_FORMATS[g.slug] ?? '1V1'}
                </span>
              </div>

              <div className="flex flex-1 flex-col gap-3 p-4">
                <div>
                  <h3 className="text-[15px] font-bold leading-tight text-white">{g.title}</h3>
                  <p className="mt-1 line-clamp-1 text-xs text-[#9ca1a8]">
                    {GAME_TAGLINES[g.slug] ?? g.description}
                  </p>
                </div>

                <div className="flex items-center gap-3 text-[11px] text-[#7c8189]">
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {g.default_turn_seconds}s turn
                  </span>
                  <span className="flex items-center gap-1">
                    <Users className="h-3 w-3" />
                    {g.max_players} players
                  </span>
                </div>

                <div className="mt-auto grid grid-cols-2 gap-2">
                  <Button
                    onClick={() => handleCreateMatch(g.slug, false, true)}
                    disabled={createMutation.isPending}
                    className="h-9 gap-1.5 px-2 text-[11px]"
                  >
                    <Swords className="h-3.5 w-3.5" />
                    Quick Play
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => openChallenge(g.slug)}
                    className="h-9 gap-1.5 border-[#2c2f36] bg-[#17181d] px-2 text-[11px] text-white hover:bg-[#1e2027] hover:text-white"
                  >
                    <Users className="h-3.5 w-3.5" />
                    Challenge
                  </Button>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>

      {/* Crash arena is hidden from the lobby for now. The `/crash` route still
          works for anyone with a direct link; restore this block to surface it.
      <section className="relative overflow-hidden rounded-2xl border border-[#23252b] bg-[linear-gradient(115deg,#151317_0%,#121317_55%,#0f1013_100%)] p-4 sm:p-5">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-16 -top-20 h-64 w-64 rounded-full bg-[#ff5b55]/12 blur-3xl"
        />
        <div className="relative z-10 flex flex-col gap-4 sm:flex-row sm:items-center">
          <div className="h-24 w-full shrink-0 overflow-hidden rounded-xl border border-[#2a1c1e] bg-[#0c0d10] sm:w-56">
            <CrashArtwork />
          </div>
          <div className="min-w-0 flex-1">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-[#ff5b55]/25 bg-[#ff5b55]/10 px-2.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.16em] text-[#ff8a80]">
              <Zap className="h-3 w-3" />
              Shared round
            </span>
            <h3 className="mt-2 text-lg font-bold text-white">Crash</h3>
            <p className="mt-1 text-xs text-[#9ca1a8]">
              Ride the multiplier. Cash out before the round crashes.
            </p>
          </div>
          <Button
            onClick={() => navigate('/crash')}
            className="h-10 shrink-0 gap-1.5 px-5 text-sm"
          >
            Play Crash
            <ArrowUpRight className="h-4 w-4" />
          </Button>
        </div>
      </section>
      */}

      {/* Lobby / history */}
      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex w-full gap-1 rounded-xl border border-[#23252b] bg-[#121317] p-1 sm:w-auto">
            {([
              { key: 'lobby', label: `Live Lobby (${lobbyMatches.length})` },
              { key: 'history', label: 'My Match History' },
            ] as const).map(({ key, label }) => (
              <button
                key={key}
                type="button"
                onClick={() => setActiveTab(key)}
                className={cn(
                  'flex-1 whitespace-nowrap rounded-lg px-4 py-1.5 text-sm font-semibold transition-colors sm:flex-none',
                  activeTab === key
                    ? 'bg-[#ff5b55] text-white shadow-sm shadow-[#ff5b55]/25'
                    : 'text-[#9ca1a8] hover:text-white'
                )}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => navigate('/leaderboard')}
              className="h-8 gap-1.5 text-xs text-[#9ca1a8] hover:bg-white/5 hover:text-white"
            >
              <Trophy className="h-3.5 w-3.5 text-[#fbbf24]" />
              Leaderboard
            </Button>
            {activeTab === 'lobby' && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => refetchLobby()}
                className="h-8 gap-1.5 text-xs text-[#9ca1a8] hover:bg-white/5 hover:text-white"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                Refresh
              </Button>
            )}
          </div>
        </div>

        {activeTab === 'lobby' && (
          <div>
            {lobbyLoading ? (
              <div className="py-12 text-center text-sm text-muted-foreground">
                Loading open matches…
              </div>
            ) : lobbyMatches.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-[#2c2f36] bg-[#101113] p-8 text-center">
                <Users className="mx-auto mb-2 h-8 w-8 text-[#5b6068]" />
                <p className="font-semibold text-white">No open matches right now</p>
                <p className="mt-1 text-xs text-[#9ca1a8]">
                  Be the first to create an open room or start a quick match!
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                {lobbyMatches.map((m) => {
                  const isMine = user && m.created_by === user.id
                  const hostPlayer = m.players?.[0]

                  return (
                    <div
                      key={m.id}
                      className={cn(CARD, 'flex items-center justify-between gap-3 p-4')}
                    >
                      <div className="min-w-0 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <GameIcon slug={m.game_slug} />
                          <span className="text-sm font-bold capitalize text-white">
                            {m.game_slug.replace(/-/g, ' ')}
                          </span>
                          <span className="rounded-full border border-[#2c2f36] px-2 py-0.5 text-[10px] font-semibold text-[#9ca1a8]">
                            {m.max_rounds > 1 ? `Best of ${m.max_rounds}` : 'Single Round'}
                          </span>
                        </div>
                        <p className="truncate text-xs text-[#9ca1a8]">
                          Host:{' '}
                          <span className="font-medium text-white">
                            {hostPlayer?.display_name || 'Player 1'}
                          </span>
                        </p>
                      </div>

                      {isMine ? (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => navigate(`/games/match/${m.id}`)}
                          className="h-8 shrink-0 text-xs"
                        >
                          Re-enter Room
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          disabled={joinMutation.isPending}
                          onClick={() => joinMutation.mutate(m.id)}
                          className="h-8 shrink-0 text-xs"
                        >
                          Join Duel
                        </Button>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}

        {activeTab === 'history' && (
          <div>
            {historyLoading ? (
              <div className="py-12 text-center text-sm text-muted-foreground">
                Loading match history…
              </div>
            ) : myMatches.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-[#2c2f36] bg-[#101113] p-8 text-center">
                <Trophy className="mx-auto mb-2 h-8 w-8 text-[#5b6068]" />
                <p className="font-semibold text-white">No completed matches yet</p>
                <p className="mt-1 text-xs text-[#9ca1a8]">
                  Play your first match to start tracking wins and statistics!
                </p>
              </div>
            ) : (
              <div className="space-y-2">
                {myMatches.map((m) => {
                  const isWinner = m.winner_user_id === user?.id
                  const isTie = m.status === 'tied'
                  const outcome = isTie
                    ? 'Tie'
                    : isWinner
                      ? 'Victory'
                      : m.status === 'forfeited'
                        ? 'Forfeited'
                        : 'Defeat'
                  const outcomeClass = isTie
                    ? 'border-[#3a3e46] bg-[#1a1c21] text-[#c7cbd1]'
                    : isWinner
                      ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400'
                      : 'border-red-500/30 bg-red-500/10 text-red-400'

                  return (
                    <div
                      key={m.id}
                      className={cn(CARD, 'flex items-center justify-between gap-3 p-3.5')}
                    >
                      <div className="flex min-w-0 items-center gap-3">
                        <GameIcon slug={m.game_slug} size="lg" />
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-bold capitalize text-white">
                              {m.game_slug.replace(/-/g, ' ')}
                            </span>
                            <span
                              className={cn(
                                'rounded-full border px-2 py-0.5 text-[10px] font-semibold',
                                outcomeClass
                              )}
                            >
                              {outcome}
                            </span>
                          </div>
                          <p className="mt-0.5 truncate text-xs text-[#9ca1a8]">
                            Final Score: {m.scores?.['0'] ?? 0} - {m.scores?.['1'] ?? 0} •{' '}
                            {new Date(m.created_at).toLocaleDateString()}
                          </p>
                        </div>
                      </div>

                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/games/match/${m.id}`)}
                        className="h-8 shrink-0 text-xs text-[#c7cbd1] hover:bg-white/5 hover:text-white"
                      >
                        Inspect
                      </Button>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}
      </section>

      {/* Host Custom Match Modal */}
      {createModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-2xl border border-[#26282e] bg-[#121317] p-5 shadow-2xl">
            <div className="flex items-center gap-2 text-lg font-bold text-white">
              <PlusCircle className="h-5 w-5 text-[#ff5b55]" />
              Host Custom P2P Match
            </div>

            <div className="mt-4 space-y-4">
              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[#7c8189]">
                  Select Game
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {games.map((g) => (
                    <button
                      key={g.slug}
                      type="button"
                      onClick={() => setSelectedGameSlug(g.slug)}
                      className={cn(
                        'flex flex-col items-center gap-1 rounded-xl border p-2.5 text-center text-xs font-bold transition-all',
                        selectedGameSlug === g.slug
                          ? 'border-[#ff5b55]/60 bg-[#ff5b55]/10 text-[#ff8a80]'
                          : 'border-[#2c2f36] text-[#9ca1a8] hover:bg-white/5 hover:text-white'
                      )}
                    >
                      <GameIcon slug={g.slug} />
                      <span className="w-full truncate">{g.title.split(' ')[0]}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[#7c8189]">
                  Match Privacy
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setMatchType('public_queue')}
                    className={cn(
                      'rounded-xl border p-2.5 text-center text-xs font-bold transition-all',
                      matchType === 'public_queue'
                        ? 'border-[#ff5b55]/60 bg-[#ff5b55]/10 text-[#ff8a80]'
                        : 'border-[#2c2f36] text-[#9ca1a8] hover:bg-white/5 hover:text-white'
                    )}
                  >
                    Public Queue (Anyone)
                  </button>
                  <button
                    type="button"
                    onClick={() => setMatchType('private_room')}
                    className={cn(
                      'rounded-xl border p-2.5 text-center text-xs font-bold transition-all',
                      matchType === 'private_room'
                        ? 'border-[#ff5b55]/60 bg-[#ff5b55]/10 text-[#ff8a80]'
                        : 'border-[#2c2f36] text-[#9ca1a8] hover:bg-white/5 hover:text-white'
                    )}
                  >
                    Private Code (Friends)
                  </button>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <Button variant="ghost" onClick={() => setCreateModalOpen(false)}>
                  Cancel
                </Button>
                <Button
                  disabled={createMutation.isPending}
                  onClick={() => handleCreateMatch(selectedGameSlug, matchType === 'private_room')}
                >
                  {createMutation.isPending ? 'Creating…' : 'Create Room'}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {challengeModalOpen && (
        <ChallengeFriendModal
          open={challengeModalOpen}
          onClose={() => setChallengeModalOpen(false)}
          games={games}
          initialGameSlug={challengeGameSlug}
        />
      )}

      {acceptCode && (
        <AcceptInviteModal
          inviteCode={acceptCode}
          onClose={handleCloseAccept}
          onOpenChallengeModal={() => openChallenge()}
        />
      )}
    </div>
  )
}

export default Component
