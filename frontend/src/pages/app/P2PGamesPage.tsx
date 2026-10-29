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
  Sparkles,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { useP2PStream } from '@/features/p2p/useP2PStream'
import { GameIcon } from '@/features/p2p/GameIcon'
import { ChallengeFriendModal } from '@/features/p2p/ChallengeFriendModal'
import { AcceptInviteModal } from '@/features/p2p/AcceptInviteModal'
import { extractStartInviteCode } from '@/lib/telegram'

function apiErrorMessage(err: unknown, fallback = 'Something went wrong'): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return fallback
}

export function Component() {
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

  return (
    <div className="mx-auto max-w-6xl py-8 px-4 space-y-8">
      {/* Hero Header */}
      <div className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-indigo-950 via-slate-900 to-slate-950 border border-indigo-500/20 p-6 sm:p-10 shadow-2xl">
        <div className="relative z-10 max-w-2xl space-y-4">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-500/20 border border-indigo-500/30 text-indigo-300 text-xs font-semibold">
            <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
            <span>P2P Competitive Arenas</span>
          </div>

          <h1 className="text-3xl sm:text-4xl md:text-5xl font-black tracking-tight text-white">
            Play Live Against Other Players
          </h1>

          <p className="text-slate-300 text-sm sm:text-base leading-relaxed">
            Test your skills in fast-paced turn-based games. Join public matchmaking or create a private room to challenge your friends with a code.
          </p>

          <div className="pt-2 flex flex-wrap items-center gap-3">
            <Button
              onClick={() => {
                setChallengeGameSlug(undefined)
                setChallengeModalOpen(true)
              }}
              className="gap-2 font-bold bg-primary hover:bg-primary/90 text-primary-foreground shadow-lg shadow-primary/25 h-11 px-5 text-sm"
            >
              <Swords className="w-4 h-4" />
              <span>Challenge a Friend</span>
            </Button>
            <Button
              variant="outline"
              onClick={() => setCreateModalOpen(true)}
              className="gap-1.5 h-11 px-4 text-sm font-semibold border-white/20 bg-white/5 hover:bg-white/10 text-white"
            >
              <PlusCircle className="w-4 h-4" />
              <span>Host Custom Match</span>
            </Button>
          </div>
        </div>

        {/* Decorative background glow */}
        <div className="absolute right-0 top-0 w-96 h-96 bg-indigo-500/10 rounded-full blur-3xl pointer-events-none" />
      </div>

      {/* Join with Invite Code Bar */}
      <Card className="border-border/80 shadow-sm bg-card/60 backdrop-blur">
        <CardContent className="p-4 sm:p-6">
          <form onSubmit={handleJoinByCode} className="flex flex-col sm:flex-row items-center gap-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground whitespace-nowrap">
              <span>Have a Private Invite Code?</span>
            </div>
            <div className="flex-1 w-full flex items-center gap-2">
              <Input
                placeholder="Enter 6-character code (e.g. ABC9X2)"
                value={inviteCodeInput}
                onChange={(e) => setInviteCodeInput(e.target.value.toUpperCase())}
                maxLength={8}
                className="font-mono uppercase tracking-wider text-center sm:text-left h-10"
              />
              <Button
                type="submit"
                disabled={!inviteCodeInput.trim()}
                className="h-10 px-6 font-semibold shrink-0 gap-1.5"
              >
               <span>Preview & Join</span>
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Games Catalog */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Gamepad2 className="w-5 h-5 text-primary" />
            <h2 className="text-xl font-bold tracking-tight">Available Games</h2>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              onClick={() => {
                setChallengeGameSlug(undefined)
                setChallengeModalOpen(true)
              }}
              className="gap-1.5 font-semibold text-xs h-8"
            >
              <Swords className="w-3.5 h-3.5" />
              <span>Challenge Friend</span>
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCreateModalOpen(true)}
              className="gap-1.5 text-xs h-8"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Host Custom Match</span>
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          {games.map((g) => {
            return (
              <Card
                key={g.slug}
                className="group relative overflow-hidden border-border/80 hover:border-primary/50 transition-all hover:shadow-xl flex flex-col justify-between"
              >
                <CardHeader className="space-y-3 pb-3">
                  <div className="flex items-center justify-between">
                    <span className="p-2 rounded-xl bg-muted/60">
                      <GameIcon slug={g.slug} size="lg" />
                    </span>
                    <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10">
                      Free Play
                    </Badge>
                  </div>
                  <div>
                    <CardTitle className="text-lg font-bold group-hover:text-primary transition-colors">
                      {g.title}
                    </CardTitle>
                    <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                      {g.description}
                    </p>
                  </div>
                </CardHeader>

                <CardContent className="pt-0 space-y-4">
                  <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground border-t border-border/40 pt-3">
                    <div className="flex items-center gap-1.5">
                      <Clock className="w-3.5 h-3.5" />
                      <span>{g.default_turn_seconds}s turn clock</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <Users className="w-3.5 h-3.5" />
                      <span>{g.max_players} Players (1v1)</span>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-2 pt-1">
                    <Button
                      onClick={() => handleCreateMatch(g.slug, false, true)}
                      disabled={createMutation.isPending}
                      className="w-full gap-1.5 h-9 font-semibold text-xs"
                    >
                      <Swords className="w-3.5 h-3.5" />
                      <span>Quick Play</span>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => {
                        setChallengeGameSlug(g.slug)
                        setChallengeModalOpen(true)
                      }}
                      className="w-full gap-1.5 h-9 font-semibold text-xs"
                    >
                      <Users className="w-3.5 h-3.5" />
                      <span>Challenge</span>
                    </Button>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      </div>

      {/* Tabs: Public Lobby vs Match History */}
      <div className="space-y-4">
        <div className="flex items-center justify-between border-b border-border pb-2">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setActiveTab('lobby')}
              className={[
                'px-4 py-1.5 text-sm font-semibold rounded-lg transition-colors',
                activeTab === 'lobby'
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground',
              ].join(' ')}
            >
              Live Lobby ({lobbyMatches.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('history')}
              className={[
                'px-4 py-1.5 text-sm font-semibold rounded-lg transition-colors',
                activeTab === 'history'
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground',
              ].join(' ')}
            >
              My Match History
            </button>
            <button
              type="button"
              onClick={() => navigate('/leaderboard')}
              className="flex items-center gap-1.5 px-4 py-1.5 text-sm font-semibold rounded-lg transition-colors text-muted-foreground hover:text-foreground hover:bg-accent/50"
            >
              <Trophy className="h-4 w-4 text-amber-500" />
              <span>Leaderboard</span>
            </button>
          </div>

          {activeTab === 'lobby' && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => refetchLobby()}
              className="gap-1.5 text-xs text-muted-foreground h-8"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              <span>Refresh</span>
            </Button>
          )}
        </div>

        {/* Tab 1: Live Lobby */}
        {activeTab === 'lobby' && (
          <div>
            {lobbyLoading ? (
              <div className="text-center py-12 text-muted-foreground text-sm">
                Loading open matches…
              </div>
            ) : lobbyMatches.length === 0 ? (
              <Card className="border-dashed p-8 text-center text-muted-foreground">
                <Users className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="font-semibold text-foreground">No open matches right now</p>
                <p className="text-xs mt-1">Be the first to create an open room or start a quick match!</p>
              </Card>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {lobbyMatches.map((m) => {
                  const isMine = user && m.created_by === user.id
                  const hostPlayer = m.players?.[0]

                  return (
                    <Card key={m.id} className="border-border/80 shadow-sm flex items-center justify-between p-4">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <GameIcon slug={m.game_slug} />
                          <span className="font-bold text-sm capitalize">{m.game_slug.replace('-', ' ')}</span>
                          <Badge variant="secondary" className="text-[10px]">
                            {m.max_rounds > 1 ? `Best of ${m.max_rounds}` : 'Single Round'}
                          </Badge>
                        </div>
                        <p className="text-xs text-muted-foreground">
                          Host: <span className="font-medium text-foreground">{hostPlayer?.display_name || 'Player 1'}</span>
                        </p>
                      </div>

                      <div>
                        {isMine ? (
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => navigate(`/games/match/${m.id}`)}
                            className="text-xs h-8"
                          >
                            Re-enter Room
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            disabled={joinMutation.isPending}
                            onClick={() => joinMutation.mutate(m.id)}
                            className="text-xs h-8 font-semibold"
                          >
                            Join Duel
                          </Button>
                        )}
                      </div>
                    </Card>
                  )
                })}
              </div>
            )}
          </div>
        )}

        {/* Tab 2: Match History */}
        {activeTab === 'history' && (
          <div>
            {historyLoading ? (
              <div className="text-center py-12 text-muted-foreground text-sm">Loading match history…</div>
            ) : myMatches.length === 0 ? (
              <Card className="border-dashed p-8 text-center text-muted-foreground">
                <Trophy className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="font-semibold text-foreground">No completed matches yet</p>
                <p className="text-xs mt-1">Play your first match to start tracking wins and statistics!</p>
              </Card>
            ) : (
              <div className="space-y-2">
                {myMatches.map((m) => {
                  const isWinner = m.winner_user_id === user?.id
                  const isTie = m.status === 'tied'

                  return (
                    <Card key={m.id} className="border-border/80 p-3.5 flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <GameIcon slug={m.game_slug} size="lg" />
                        <div>
                          <div className="flex items-center gap-2">
                            <span className="font-bold text-sm capitalize">{m.game_slug.replace('-', ' ')}</span>
                            <Badge
                              variant={
                                isTie
                                  ? 'secondary'
                                  : isWinner
                                    ? 'default'
                                    : 'destructive'
                              }
                              className="text-[10px]"
                            >
                              {isTie ? 'Tie' : isWinner ? 'Victory' : m.status === 'forfeited' ? 'Forfeited' : 'Defeat'}
                            </Badge>
                          </div>
                          <p className="text-xs text-muted-foreground mt-0.5">
                            Final Score: {m.scores?.['0'] ?? 0} - {m.scores?.['1'] ?? 0} • {new Date(m.created_at).toLocaleDateString()}
                          </p>
                        </div>
                      </div>

                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/games/match/${m.id}`)}
                        className="text-xs h-8"
                      >
                        Inspect
                      </Button>
                    </Card>
                  )
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Host Custom Match Modal */}
      {createModalOpen && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <Card className="max-w-md w-full border-border/80 shadow-2xl animate-in fade-in zoom-in-95">
            <CardHeader>
              <CardTitle className="text-lg font-bold flex items-center gap-2">
                <PlusCircle className="w-5 h-5 text-primary" />
                <span>Host Custom P2P Match</span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  Select Game
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {games.map((g) => (
                    <button
                      key={g.slug}
                      type="button"
                      onClick={() => setSelectedGameSlug(g.slug)}
                      className={[
                        'p-2.5 rounded-xl border text-center transition-all text-xs font-bold flex flex-col items-center gap-1',
                        selectedGameSlug === g.slug
                          ? 'border-primary bg-primary/10 text-primary'
                          : 'border-border/80 hover:bg-muted/50',
                      ].join(' ')}
                    >
                      <GameIcon slug={g.slug} />
                      <span className="truncate w-full">{g.title.split(' ')[0]}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  Match Privacy
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setMatchType('public_queue')}
                    className={[
                      'p-2.5 rounded-xl border text-center text-xs font-bold transition-all',
                      matchType === 'public_queue'
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border/80 hover:bg-muted/50',
                    ].join(' ')}
                  >
                    Public Queue (Anyone)
                  </button>
                  <button
                    type="button"
                    onClick={() => setMatchType('private_room')}
                    className={[
                      'p-2.5 rounded-xl border text-center text-xs font-bold transition-all',
                      matchType === 'private_room'
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border/80 hover:bg-muted/50',
                    ].join(' ')}
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
                  className="font-semibold"
                >
                  {createMutation.isPending ? 'Creating…' : 'Create Room'}
                </Button>
              </div>
            </CardContent>
          </Card>
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
          onOpenChallengeModal={() => {
            setChallengeGameSlug(undefined)
            setChallengeModalOpen(true)
          }}
        />
      )}
    </div>
  )
}

export default Component
