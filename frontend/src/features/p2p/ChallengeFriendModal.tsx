import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  X,
  Swords,
  Copy,
  Check,
  Send,
  Share2,
  Clock,
  ArrowRight,
  ArrowLeft,
  AlertTriangle,
  Loader2,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { GameIcon } from '@/features/p2p/GameIcon'
import {
  buildInviteUrl,
  buildWebInviteUrl,
  buildInviteMessage,
  shareViaTelegram,
  shareLinkOrCopy,
  copyToClipboard,
} from '@/lib/telegram'
import type { P2PGame, P2PMatch } from '@/types/api'

function apiErrorMessage(err: unknown, fallback = 'Something went wrong'): string {
  if (typeof err === 'object' && err !== null) {
    const response = (err as { response?: { data?: { message?: string } } }).response
    if (response?.data?.message) return response.data.message
    const message = (err as { message?: string }).message
    if (message) return message
  }
  return fallback
}

interface ChallengeFriendModalProps {
  open: boolean
  onClose: () => void
  games: P2PGame[]
  initialGameSlug?: string
}

export function ChallengeFriendModal({
  open,
  onClose,
  games,
  initialGameSlug,
}: ChallengeFriendModalProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const defaultGame = games.find((g) => g.slug === initialGameSlug) || games[0] || null
  const [selectedGameSlug, setSelectedGameSlug] = useState<string>(
    initialGameSlug || games[0]?.slug || 'tic-tac-toe'
  )
  const [step, setStep] = useState<'select' | 'configure' | 'share'>(
    initialGameSlug ? 'configure' : 'select'
  )
  const [createdMatch, setCreatedMatch] = useState<P2PMatch | null>(null)
  const [copiedCode, setCopiedCode] = useState(false)
  const [copiedLink, setCopiedLink] = useState(false)
  const [secondsRemaining, setSecondsRemaining] = useState(300) // 5 minutes TTL
  const [showCancelConfirm, setShowCancelConfirm] = useState(false)

  const [prevGameSlug, setPrevGameSlug] = useState(initialGameSlug)
  if (initialGameSlug !== prevGameSlug) {
    setPrevGameSlug(initialGameSlug)
    if (initialGameSlug) {
      setSelectedGameSlug(initialGameSlug)
      setStep('configure')
    }
  }

  // Countdown timer when match is created using backend expiration
  useEffect(() => {
    if (step !== 'share' || !createdMatch) return

    const expiresMs = createdMatch.expires_at
      ? new Date(createdMatch.expires_at).getTime()
      : new Date(createdMatch.created_at).getTime() + 5 * 60 * 1000

    const interval = setInterval(() => {
      const remaining = Math.max(0, Math.floor((expiresMs - Date.now()) / 1000))
      setSecondsRemaining(remaining)
    }, 1000)
    return () => clearInterval(interval)
  }, [step, createdMatch])

  const selectedGame = games.find((g) => g.slug === selectedGameSlug) || defaultGame

  // Mutation to create a private friend match
  const createMatchMutation = useMutation({
    mutationFn: () =>
      p2pApi.createMatch({
        game_slug: selectedGameSlug,
        match_type: 'private_room',
        stake_amount: 0,
        auto_match: false,
      }),
    onSuccess: (match) => {
      queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      queryClient.invalidateQueries({ queryKey: ['p2p-my-matches'] })
      setCreatedMatch(match)
      setStep('share')
      toast({
        title: 'Challenge Created!',
        description: 'Share your room link or code with your friend to begin.',
        type: 'success',
      })
    },
    onError: (err: unknown) => {
      toast({
        title: 'Could not create challenge',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  // Mutation to cancel the waiting match
  const cancelMatchMutation = useMutation({
    mutationFn: (matchId: string) => p2pApi.cancelMatch(matchId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      queryClient.invalidateQueries({ queryKey: ['p2p-my-matches'] })
      toast({ title: 'Challenge Cancelled', type: 'info' })
      onClose()
    },
    onError: (err: unknown) => {
      toast({
        title: 'Error cancelling match',
        description: apiErrorMessage(err),
        type: 'error',
      })
    },
  })

  if (!open) return null

  const handleCopyCode = async () => {
    if (!createdMatch?.invite_code) return
    const success = await copyToClipboard(createdMatch.invite_code)
    if (success) {
      setCopiedCode(true)
      setTimeout(() => setCopiedCode(false), 2000)
      toast({ title: 'Code Copied!', description: '6-character room code copied to clipboard.', type: 'info' })
    } else {
      toast({ title: 'Copy Failed', description: 'Please copy the code manually.', type: 'error' })
    }
  }

  const handleCopyLink = async () => {
    if (!createdMatch?.invite_code) return
    const link = buildWebInviteUrl(createdMatch.invite_code)
    const success = await copyToClipboard(link)
    if (success) {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({ title: 'Link Copied!', description: 'Challenge link copied to clipboard.', type: 'info' })
    } else {
      toast({ title: 'Copy Failed', description: 'Please copy the link manually.', type: 'error' })
    }
  }

  const handleShareTelegram = () => {
    if (!createdMatch?.invite_code || !selectedGame) return
    const inviteUrl = buildInviteUrl(createdMatch.invite_code)
    const message = buildInviteMessage(selectedGame.title, createdMatch.invite_code, inviteUrl)
    shareViaTelegram(inviteUrl, message)
  }

  const handleNativeShare = async () => {
    if (!createdMatch?.invite_code || !selectedGame) return
    const inviteUrl = buildWebInviteUrl(createdMatch.invite_code)
    const message = buildInviteMessage(selectedGame.title, createdMatch.invite_code, inviteUrl)
    const result = await shareLinkOrCopy({
      title: `Challenge: ${selectedGame.title}`,
      text: message,
      url: inviteUrl,
    })
    if (result === 'copied') {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({ title: 'Link Copied!', description: 'Challenge link copied to clipboard.', type: 'info' })
    }
  }

  const handleEnterArena = () => {
    if (!createdMatch) return
    onClose()
    navigate(`/games/match/${createdMatch.id}`)
  }

  const handleCancelChallenge = () => {
    if (createdMatch) {
      cancelMatchMutation.mutate(createdMatch.id)
    } else {
      onClose()
    }
  }

  const formatCountdown = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs < 10 ? '0' : ''}${secs}`
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm animate-in fade-in duration-200"
      role="dialog"
      aria-modal="true"
      aria-labelledby="challenge-modal-title"
    >
      <Card className="w-full max-w-lg border-border/80 shadow-2xl overflow-hidden bg-card/95 backdrop-blur">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border/70 px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Swords className="h-5 w-5" />
            </div>
            <div>
              <h2 id="challenge-modal-title" className="text-base font-bold text-foreground">
                {step === 'share' ? 'Your Challenge is Ready' : 'Challenge a Friend'}
              </h2>
              <p className="text-xs text-muted-foreground">
                {step === 'select'
                  ? 'Choose a game to duel 1-on-1'
                  : step === 'configure'
                    ? 'Review rules and create private match'
                    : 'Share the invitation link or code'}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={() => {
              if (step === 'share' && createdMatch) {
                setShowCancelConfirm(true)
              } else {
                onClose()
              }
            }}
            className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <CardContent className="p-5 space-y-5">
          {/* STEP 1: SELECT GAME */}
          {step === 'select' && (
            <div className="space-y-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {games.map((g) => {
                  const isSelected = selectedGameSlug === g.slug
                  return (
                    <button
                      key={g.slug}
                      type="button"
                      onClick={() => setSelectedGameSlug(g.slug)}
                      className={[
                        'flex items-start gap-3 p-3.5 rounded-2xl border text-left transition-all',
                        isSelected
                          ? 'border-primary bg-primary/10 shadow-sm ring-1 ring-primary/40'
                          : 'border-border/70 bg-card hover:border-border hover:bg-muted/40',
                      ].join(' ')}
                    >
                      <div className="p-2 rounded-xl bg-muted/60 shrink-0">
                        <GameIcon slug={g.slug} size="md" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-1">
                          <span className="font-bold text-sm text-foreground truncate">{g.title}</span>
                          {isSelected && <Check className="h-4 w-4 text-primary shrink-0" />}
                        </div>
                        <p className="text-xs text-muted-foreground line-clamp-1 mt-0.5">
                          {g.description}
                        </p>
                        <div className="flex items-center gap-2 mt-2 text-[11px] text-muted-foreground">
                          <span className="flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {g.default_turn_seconds}s
                          </span>
                          <span>•</span>
                          <span>1v1</span>
                        </div>
                      </div>
                    </button>
                  )
                })}
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t border-border/50">
                <Button variant="ghost" size="sm" onClick={onClose}>
                  Cancel
                </Button>
                <Button
                  size="sm"
                  onClick={() => setStep('configure')}
                  className="gap-1.5 font-semibold"
                >
                  <span>Continue</span>
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}

          {/* STEP 2: CONFIGURE & CONFIRM */}
          {step === 'configure' && selectedGame && (
            <div className="space-y-4">
              {/* Selected Game Card */}
              <div className="flex items-center justify-between p-4 rounded-2xl border border-border/80 bg-muted/30">
                <div className="flex items-center gap-3">
                  <div className="p-2.5 rounded-xl bg-muted shrink-0">
                    <GameIcon slug={selectedGame.slug} size="lg" />
                  </div>
                  <div>
                    <h3 className="font-bold text-base text-foreground">{selectedGame.title}</h3>
                    <p className="text-xs text-muted-foreground mt-0.5">{selectedGame.description}</p>
                  </div>
                </div>

                {!initialGameSlug && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setStep('select')}
                    className="text-xs text-primary hover:text-primary/90 h-8 shrink-0"
                  >
                    Change
                  </Button>
                )}
              </div>

              {/* Match Settings Review */}
              <div className="grid grid-cols-2 gap-3 text-xs">
                <div className="p-3 rounded-xl border border-border/60 bg-card/60 space-y-1">
                  <span className="text-muted-foreground font-medium">Match Format</span>
                  <p className="font-bold text-foreground">
                    {selectedGame.slug === 'tic-tac-toe' || selectedGame.slug === 'rock-paper-scissors'
                      ? 'Best-of-3 Rounds'
                      : 'Single Round (First Win)'}
                  </p>
                </div>
                <div className="p-3 rounded-xl border border-border/60 bg-card/60 space-y-1">
                  <span className="text-muted-foreground font-medium">Turn Clock</span>
                  <p className="font-bold text-foreground">{selectedGame.default_turn_seconds} seconds per move</p>
                </div>
                <div className="p-3 rounded-xl border border-border/60 bg-card/60 space-y-1">
                  <span className="text-muted-foreground font-medium">Room Privacy</span>
                  <p className="font-bold text-emerald-600 dark:text-emerald-400">Private Friend Match</p>
                </div>
                <div className="p-3 rounded-xl border border-border/60 bg-card/60 space-y-1">
                  <span className="text-muted-foreground font-medium">Match Stake</span>
                  <p className="font-bold text-foreground">Free Play Mode</p>
                </div>
              </div>

              <div className="flex items-center justify-between pt-2 border-t border-border/50">
                {!initialGameSlug ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setStep('select')}
                    className="gap-1.5 text-xs text-muted-foreground"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    <span>Back</span>
                  </Button>
                ) : (
                  <Button variant="ghost" size="sm" onClick={onClose} className="text-xs">
                    Cancel
                  </Button>
                )}

                <Button
                  size="default"
                  disabled={createMatchMutation.isPending}
                  onClick={() => createMatchMutation.mutate()}
                  className="gap-2 font-bold px-6 shadow-md shadow-primary/20"
                >
                  {createMatchMutation.isPending ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <span>Creating Room…</span>
                    </>
                  ) : (
                    <>
                      <Swords className="h-4 w-4" />
                      <span>Create Challenge</span>
                    </>
                  )}
                </Button>
              </div>
            </div>
          )}

          {/* STEP 3: SHARE INVITATION */}
          {step === 'share' && createdMatch && selectedGame && (
            <div className="space-y-5 animate-in fade-in duration-300">
              {/* Waiting Status Banner */}
              <div className="flex items-center justify-between rounded-xl border border-indigo-500/30 bg-indigo-500/10 px-3.5 py-2.5 text-xs">
                <div className="flex items-center gap-2 text-indigo-400 font-semibold">
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500" />
                  </span>
                  <span>Waiting for friend to join…</span>
                </div>
                <Badge variant="outline" className="text-[11px] font-mono border-indigo-500/40 text-indigo-300">
                  <Clock className="w-3 h-3 mr-1" />
                  {formatCountdown(secondsRemaining)}
                </Badge>
              </div>

              {/* Room Code Card */}
              <div className="p-4 rounded-2xl border border-border/80 bg-muted/30 text-center space-y-2">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  Private Room Code
                </span>
                <div className="flex items-center justify-center gap-3">
                  <span className="font-mono text-3xl sm:text-4xl font-black tracking-widest text-primary selection:bg-primary selection:text-primary-foreground">
                    {createdMatch.invite_code}
                  </span>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleCopyCode}
                    className="h-9 px-3 gap-1.5 text-xs font-semibold"
                  >
                    {copiedCode ? (
                      <>
                        <Check className="h-4 w-4 text-emerald-500" />
                        <span>Copied</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-4 w-4" />
                        <span>Copy</span>
                      </>
                    )}
                  </Button>
                </div>
              </div>

              {/* Share Actions Grid */}
              <div className="space-y-2.5">
                <Button
                  onClick={handleShareTelegram}
                  className="w-full h-11 gap-2 font-bold text-sm bg-gradient-to-r from-sky-500 to-blue-600 hover:from-sky-600 hover:to-blue-700 text-white shadow-md shadow-sky-500/20"
                >
                  <Send className="h-4 w-4" />
                  <span>Invite via Telegram</span>
                </Button>

                <div className="grid grid-cols-2 gap-2">
                  <Button
                    variant="outline"
                    onClick={handleCopyLink}
                    className="h-10 text-xs font-semibold gap-1.5"
                  >
                    {copiedLink ? (
                      <>
                        <Check className="h-3.5 w-3.5 text-emerald-500" />
                        <span>Link Copied</span>
                      </>
                    ) : (
                      <>
                        <Copy className="h-3.5 w-3.5" />
                        <span>Copy Link</span>
                      </>
                    )}
                  </Button>

                  <Button
                    variant="outline"
                    onClick={handleNativeShare}
                    className="h-10 text-xs font-semibold gap-1.5"
                  >
                    <Share2 className="h-3.5 w-3.5" />
                    <span>Share Options</span>
                  </Button>
                </div>
              </div>

              {/* Footer Actions */}
              <div className="flex items-center justify-between pt-3 border-t border-border/50">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowCancelConfirm(true)}
                  disabled={cancelMatchMutation.isPending}
                  className="text-xs text-muted-foreground hover:text-destructive"
                >
                  Cancel Invitation
                </Button>

                <Button
                  size="sm"
                  onClick={handleEnterArena}
                  className="font-bold gap-1.5"
                >
                  <span>Enter Waiting Room</span>
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>

        {/* Confirmation prompt when cancelling a created match */}
        {showCancelConfirm && (
          <div className="absolute inset-0 bg-background/90 backdrop-blur-sm z-20 flex items-center justify-center p-6 animate-in fade-in">
            <div className="text-center space-y-4 max-w-xs">
              <div className="w-12 h-12 rounded-full bg-destructive/10 text-destructive flex items-center justify-center mx-auto">
                <AlertTriangle className="h-6 w-6" />
              </div>
              <div>
                <h4 className="font-bold text-base text-foreground">Cancel this challenge?</h4>
                <p className="text-xs text-muted-foreground mt-1">
                  The room code will be invalidated and your friend will not be able to join.
                </p>
              </div>
              <div className="flex justify-center gap-2 pt-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowCancelConfirm(false)}
                >
                  Keep Waiting
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={cancelMatchMutation.isPending}
                  onClick={handleCancelChallenge}
                >
                  {cancelMatchMutation.isPending ? 'Cancelling…' : 'Yes, Cancel'}
                </Button>
              </div>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
