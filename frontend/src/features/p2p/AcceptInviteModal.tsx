import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  X,
  Swords,
  Clock,
  Check,
  AlertCircle,
  Copy,
  Users,
  Loader2,
  LogIn,
} from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useAuth } from '@/contexts/AuthContext'
import { useToast } from '@/contexts/ToastContext'
import { p2pApi } from '@/features/p2p/api'
import { GameIcon } from '@/features/p2p/GameIcon'
import { parseInviteError } from '@/features/p2p/inviteError'
import { buildWebInviteUrl, copyToClipboard } from '@/lib/telegram'
import type { InviteSummary, P2PMatch } from '@/types/api'

interface AcceptInviteModalProps {
  inviteCode: string | null
  onClose: () => void
  onOpenChallengeModal?: () => void
}

export function AcceptInviteModal({
  inviteCode,
  onClose,
  onOpenChallengeModal,
}: AcceptInviteModalProps) {
  const { user } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const cleanCode = (inviteCode || '').trim().toUpperCase()
  const [secondsLeft, setSecondsLeft] = useState<number>(0)
  const [copiedLink, setCopiedLink] = useState(false)

  // Query invitation details
  const {
    data: invite,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery<InviteSummary>({
    queryKey: ['p2p-invite', cleanCode],
    queryFn: () => p2pApi.getInvite(cleanCode),
    enabled: !!cleanCode,
    retry: 1,
  })

  // Ticking countdown based on authoritative expires_at
  useEffect(() => {
    if (!invite?.expires_at && !invite?.seconds_left) return

    const expiresMs = invite?.expires_at ? new Date(invite.expires_at).getTime() : 0
    const interval = setInterval(() => {
      if (expiresMs > 0) {
        setSecondsLeft(Math.max(0, Math.floor((expiresMs - Date.now()) / 1000)))
      } else {
        setSecondsLeft((prev) => Math.max(0, prev - 1))
      }
    }, 1000)
    return () => clearInterval(interval)
  }, [invite?.expires_at, invite?.seconds_left])

  // Join match mutation
  const joinMutation = useMutation({
    mutationFn: () => p2pApi.joinMatchByCode(cleanCode),
    onSuccess: (match: P2PMatch) => {
      queryClient.invalidateQueries({ queryKey: ['p2p-lobby'] })
      queryClient.invalidateQueries({ queryKey: ['p2p-my-matches'] })
      toast({
        title: 'Joined Match!',
        description: 'Entering the duel arena now…',
        type: 'success',
      })
      onClose()
      navigate(`/games/match/${match.id}`)
    },
    onError: (err: unknown) => {
      const errInfo = parseInviteError(err, cleanCode)
      toast({
        title: errInfo.title === 'Challenge Not Found' ? 'Failed to Join Match' : errInfo.title,
        description: errInfo.description,
        type: 'error',
      })
    },
  })

  if (!inviteCode) return null

  const displaySecondsLeft = secondsLeft > 0 ? secondsLeft : (invite?.seconds_left ?? 0)

  const isCreator = user && invite && invite.creator_id === user.id
  const isExpired = invite ? invite.is_expired || (invite.status === 'waiting' && displaySecondsLeft <= 0) : false
  const isCancelled = invite?.status === 'cancelled'
  const isFull = invite && !invite.is_joinable && !isExpired && !isCancelled
  const isJoinable = invite?.is_joinable && !isExpired && !isCreator

  const formatCountdown = (secs: number) => {
    const mins = Math.floor(secs / 60)
    const s = secs % 60
    return `${mins}:${s < 10 ? '0' : ''}${s}`
  }

  const handleCopyLink = async () => {
    const link = buildWebInviteUrl(cleanCode)
    const success = await copyToClipboard(link)
    if (success) {
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
      toast({ title: 'Link Copied!', type: 'info' })
    } else {
      toast({ title: 'Copy Failed', description: 'Please copy the link manually.', type: 'error' })
    }
  }

  const handleLoginToAccept = () => {
    const redirectUrl = `/games?join=${cleanCode}`
    if (typeof sessionStorage !== 'undefined') {
      sessionStorage.setItem('pending_redirect', redirectUrl)
    }
    onClose()
    navigate(`/login?redirect=${encodeURIComponent(redirectUrl)}`)
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm animate-in fade-in duration-200"
      role="dialog"
      aria-modal="true"
      aria-labelledby="accept-invite-title"
    >
      <Card className="w-full max-w-md border-border/80 shadow-2xl overflow-hidden bg-card/95 backdrop-blur">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border/70 px-5 py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Swords className="h-5 w-5" />
            </div>
            <div>
              <h2 id="accept-invite-title" className="text-base font-bold text-foreground">
                Match Challenge
              </h2>
              <p className="text-xs text-muted-foreground font-mono">Room Code: {cleanCode}</p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <CardContent className="p-5 space-y-4">
          {/* Loading state */}
          {isLoading && (
            <div className="py-12 flex flex-col items-center justify-center gap-3 text-center">
              <Loader2 className="w-8 h-8 animate-spin text-primary" />
              <p className="text-sm text-muted-foreground">Looking up challenge details…</p>
            </div>
          )}

          {/* Error / Not Found state */}
          {isError && (() => {
            const errInfo = parseInviteError(error, cleanCode)
            return (
              <div className="py-6 text-center space-y-3">
                <div className="w-12 h-12 rounded-full bg-destructive/10 text-destructive flex items-center justify-center mx-auto">
                  <AlertCircle className="w-6 h-6" />
                </div>
                <div>
                  <h3 className="font-bold text-base text-foreground">{errInfo.title}</h3>
                  <p className="text-xs text-muted-foreground mt-1 max-w-xs mx-auto">
                    {errInfo.description}
                  </p>
                </div>
                <div className="pt-2 flex flex-wrap items-center justify-center gap-2">
                  {errInfo.canRetry && (
                    <Button size="sm" variant="outline" onClick={() => refetch()}>
                      Try Again
                    </Button>
                  )}
                  {errInfo.isExpired && onOpenChallengeModal && (
                    <Button
                      size="sm"
                      onClick={() => {
                        onClose()
                        onOpenChallengeModal()
                      }}
                    >
                      Create Challenge
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" onClick={onClose}>
                    Back to Games
                  </Button>
                </div>
              </div>
            )
          })()}

          {/* Invitation Loaded */}
          {invite && !isLoading && !isError && (
            <div className="space-y-4 animate-in fade-in">
              {/* Challenger Profile Card */}
              <div className="flex items-center gap-3 p-3.5 rounded-2xl border border-border/80 bg-muted/40">
                <div className="relative">
                  <div className="h-11 w-11 rounded-full bg-primary/15 border-2 border-primary/30 flex items-center justify-center overflow-hidden font-bold text-primary text-base">
                    {invite.creator_avatar_url ? (
                      <img
                        src={invite.creator_avatar_url}
                        alt=""
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      invite.creator_name.charAt(0).toUpperCase()
                    )}
                  </div>
                </div>
                <div className="min-w-0 flex-1">
                  <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    Challenger
                  </span>
                  <p className="font-bold text-sm text-foreground truncate">
                    {invite.creator_name}
                  </p>
                </div>
                <Badge
                  variant={isExpired ? 'destructive' : isCancelled ? 'outline' : 'secondary'}
                  className="text-[11px]"
                >
                  {isExpired
                    ? 'Expired'
                    : isCancelled
                      ? 'Cancelled'
                      : isFull
                        ? 'Match Full'
                        : 'Open Challenge'}
                </Badge>
              </div>

              {/* Game Info Card */}
              <div className="p-4 rounded-2xl border border-border/80 bg-card space-y-3">
                <div className="flex items-center gap-3">
                  <div className="p-2.5 rounded-xl bg-muted/70 shrink-0">
                    <GameIcon slug={invite.game_slug} size="lg" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="font-bold text-base text-foreground truncate">
                      {invite.game_title}
                    </h3>
                    <p className="text-xs text-muted-foreground line-clamp-1 mt-0.5">
                      {invite.game_description}
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2 text-xs border-t border-border/50 pt-2.5 text-muted-foreground">
                  <div className="flex items-center gap-1.5">
                    <Users className="w-3.5 h-3.5 text-primary" />
                    <span>
                      {invite.max_rounds > 1 ? `Best of ${invite.max_rounds}` : 'Single Round'}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Clock className="w-3.5 h-3.5 text-primary" />
                    <span>{invite.default_turn_seconds}s turn clock</span>
                  </div>
                </div>
              </div>

              {/* Status Message based on conditions */}
              {isExpired && (
                <div className="p-3 rounded-xl border border-destructive/30 bg-destructive/10 text-destructive text-xs space-y-1">
                  <p className="font-semibold">This invitation has expired</p>
                  <p className="text-muted-foreground text-[11px]">
                    Private rooms expire after 5 minutes of inactivity. You can start a fresh match
                    with your friend instead.
                  </p>
                </div>
              )}

              {isCancelled && (
                <div className="p-3 rounded-xl border border-muted-foreground/30 bg-muted text-foreground text-xs space-y-1">
                  <p className="font-semibold">This invitation was cancelled</p>
                  <p className="text-muted-foreground text-[11px]">
                    The host closed this challenge before it was accepted.
                  </p>
                </div>
              )}

              {isFull && (
                <div className="p-3 rounded-xl border border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300 text-xs space-y-1">
                  <p className="font-semibold">Room is already full</p>
                  <p className="text-muted-foreground text-[11px]">
                    Another player already joined this 1v1 duel.
                  </p>
                </div>
              )}

              {isCreator && (
                <div className="p-3 rounded-xl border border-indigo-500/30 bg-indigo-500/10 text-indigo-400 text-xs space-y-2">
                  <p className="font-semibold">You created this challenge!</p>
                  <p className="text-muted-foreground text-[11px]">
                    Share this code with your friend or re-enter the arena to wait for them.
                  </p>
                  <div className="flex gap-2 pt-1">
                    <Button size="sm" variant="outline" onClick={handleCopyLink} className="h-8 text-xs gap-1.5">
                      {copiedLink ? <Check className="w-3.5 h-3.5 text-emerald-500" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>{copiedLink ? 'Copied' : 'Copy Share Link'}</span>
                    </Button>
                    <Button
                      size="sm"
                      onClick={() => {
                        onClose()
                        navigate(`/games/match/${invite.match_id}`)
                      }}
                      className="h-8 text-xs font-semibold"
                    >
                      Enter Room
                    </Button>
                  </div>
                </div>
              )}

              {/* Ticking countdown for active joinable room */}
              {isJoinable && displaySecondsLeft > 0 && (
                <div className="flex items-center justify-between text-xs text-muted-foreground px-1">
                  <span>Invitation active</span>
                  <span className="font-mono font-semibold text-amber-500">
                    Expires in {formatCountdown(displaySecondsLeft)}
                  </span>
                </div>
              )}

              {/* Action Buttons */}
              <div className="pt-2 flex flex-col gap-2">
                {isJoinable ? (
                  user ? (
                    <Button
                      size="lg"
                      disabled={joinMutation.isPending}
                      onClick={() => joinMutation.mutate()}
                      className="w-full font-bold text-sm gap-2 shadow-lg shadow-primary/25 h-11"
                    >
                      {joinMutation.isPending ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          <span>Joining Match…</span>
                        </>
                      ) : (
                        <>
                          <Swords className="w-4 h-4" />
                          <span>Accept Challenge ⚔️</span>
                        </>
                      )}
                    </Button>
                  ) : (
                    <Button
                      size="lg"
                      onClick={handleLoginToAccept}
                      className="w-full font-bold text-sm gap-2 shadow-lg shadow-primary/25 h-11"
                    >
                      <LogIn className="w-4 h-4" />
                      <span>Sign In to Accept Challenge</span>
                    </Button>
                  )
                ) : (isExpired || isCancelled || isFull) && onOpenChallengeModal ? (
                  <Button
                    size="default"
                    onClick={() => {
                      onClose()
                      onOpenChallengeModal()
                    }}
                    className="w-full font-bold gap-2"
                  >
                    <Swords className="w-4 h-4" />
                    <span>Create a New Challenge</span>
                  </Button>
                ) : null}

                <Button variant="ghost" size="sm" onClick={onClose} className="text-xs text-muted-foreground">
                  Close
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
