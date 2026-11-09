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
import { useTranslation } from 'react-i18next'
import { p2pApi } from '@/features/p2p/api'
import { GameIcon } from '@/features/p2p/GameIcon'
import { gameName } from '@/features/p2p/matchHistory'
import { parseInviteError } from '@/features/p2p/inviteError'
import { buildWebInviteUrl, copyToClipboard } from '@/lib/telegram'
import { formatCurrency } from '@/lib/utils'
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
  const { t } = useTranslation()
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
        title: t('p2p.acceptInvite.joinedSuccess', 'Joined Match!'),
        description: t('p2p.acceptInvite.enteringArena', 'Entering the duel arena now…'),
        type: 'success',
      })
      onClose()
      navigate(`/games/match/${match.id}`)
    },
    onError: (err: unknown) => {
      const errInfo = parseInviteError(err, cleanCode, t)
      toast({
        title: errInfo.title,
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
      toast({ title: t('p2p.challenge.linkCopied', 'Link Copied!'), type: 'info' })
    } else {
      toast({
        title: t('p2p.challenge.copyFailed', 'Copy Failed'),
        description: t('p2p.challenge.copyLinkFailedDesc', 'Please copy the link manually.'),
        type: 'error',
      })
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
                {t('p2p.acceptInvite.matchChallenge', 'Match Challenge')}
              </h2>
              <p className="text-xs text-muted-foreground font-mono">
                {t('p2p.acceptInvite.roomCode', { code: cleanCode, defaultValue: `Room Code: ${cleanCode}` })}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            aria-label={t('common.close', 'Close')}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <CardContent className="p-5 space-y-4">
          {/* Loading state */}
          {isLoading && (
            <div className="py-12 flex flex-col items-center justify-center gap-3 text-center">
              <Loader2 className="w-8 h-8 animate-spin text-primary" />
              <p className="text-sm text-muted-foreground">
                {t('p2p.acceptInvite.lookingUp', 'Looking up challenge details…')}
              </p>
            </div>
          )}

          {/* Error / Not Found state */}
          {isError && (() => {
            const errInfo = parseInviteError(error, cleanCode, t)
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
                      {t('p2p.acceptInvite.tryAgain', 'Try Again')}
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
                      {t('p2p.acceptInvite.createChallenge', 'Create Challenge')}
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" onClick={onClose}>
                    {t('p2p.acceptInvite.backToGames', 'Back to Games')}
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
                    {t('p2p.acceptInvite.challenger', 'Challenger')}
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
                    ? t('p2p.acceptInvite.expiredBadge', 'Expired')
                    : isCancelled
                      ? t('p2p.acceptInvite.cancelledBadge', 'Cancelled')
                      : isFull
                        ? t('p2p.acceptInvite.matchFullBadge', 'Match Full')
                        : t('p2p.acceptInvite.openChallengeBadge', 'Open Challenge')}
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
                      {gameName(invite.game_slug, t) || invite.game_title}
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
                      {invite.max_rounds > 1
                        ? t('p2p.acceptInvite.bestOfRounds', { rounds: invite.max_rounds, defaultValue: `Best of ${invite.max_rounds}` })
                        : t('p2p.acceptInvite.singleRound', 'Single Round')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Clock className="w-3.5 h-3.5 text-primary" />
                    <span>
                      {t('p2p.acceptInvite.turnClockSeconds', { seconds: invite.default_turn_seconds, defaultValue: `${invite.default_turn_seconds}s turn clock` })}
                    </span>
                  </div>
                </div>

                {invite.competition_mode === 'earn' ? (
                  <div className="space-y-1.5 rounded-xl border border-[#fbbf24]/30 bg-[#fbbf24]/5 p-3 text-xs">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-[#b8860b] dark:text-[#fbbf24]">
                        {t('p2p.stakeSetup.title', 'Compete to Earn')}
                      </span>
                      <span className="rounded-full border border-[#fbbf24]/40 px-2 py-0.5 text-[10px] font-bold text-[#b8860b] dark:text-[#fbbf24]">
                        {t('p2p.acceptInvite.stakeEach', {
                          amount: formatCurrency(invite.stake_amount),
                          defaultValue: `${formatCurrency(invite.stake_amount)} stake each`,
                        })}
                      </span>
                    </div>
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span>{t('p2p.stakeSetup.totalPrizePool', 'Prize pool')}</span>
                      <span className="font-semibold text-foreground">
                        {formatCurrency(invite.prize_pool)}
                      </span>
                    </div>
                    <div className="flex items-center justify-between text-muted-foreground">
                      <span>
                        {t('p2p.stakeSetup.platformFee', {
                          percent: invite.service_fee_percent.toFixed(2),
                          defaultValue: `Platform fee (${invite.service_fee_percent.toFixed(2)}%)`,
                        })}
                      </span>
                      <span className="font-semibold text-foreground">
                        {formatCurrency(invite.prize_pool - invite.potential_payout)}
                      </span>
                    </div>
                    <div className="flex items-center justify-between border-t border-[#fbbf24]/20 pt-1.5">
                      <span className="text-muted-foreground">{t('p2p.stakeSetup.potentialWinnerPayout', 'Winner payout')}</span>
                      <span className="font-bold text-success">
                        {formatCurrency(invite.potential_payout)}
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center justify-between rounded-xl border border-border/60 bg-muted/30 px-3 py-2 text-xs">
                    <span className="font-semibold text-foreground">{t('p2p.acceptInvite.freePlay', 'Free Play')}</span>
                    <span className="text-muted-foreground">{t('p2p.acceptInvite.noStakeNeeded', 'No stake or wallet balance needed')}</span>
                  </div>
                )}
              </div>

              {/* Status Message based on conditions */}
              {isExpired && (
                <div className="p-3 rounded-xl border border-destructive/30 bg-destructive/10 text-destructive text-xs space-y-1">
                  <p className="font-semibold">{t('p2p.acceptInvite.expiredTitle', 'This invitation has expired')}</p>
                  <p className="text-muted-foreground text-[11px]">
                    {t('p2p.acceptInvite.expiredDetail', 'Private rooms expire after 5 minutes of inactivity. You can start a fresh match with your friend instead.')}
                  </p>
                </div>
              )}

              {isCancelled && (
                <div className="p-3 rounded-xl border border-muted-foreground/30 bg-muted text-foreground text-xs space-y-1">
                  <p className="font-semibold">{t('p2p.acceptInvite.cancelledTitle', 'This invitation was cancelled')}</p>
                  <p className="text-muted-foreground text-[11px]">
                    {t('p2p.acceptInvite.cancelledDetail', 'The host closed this challenge before it was accepted.')}
                  </p>
                </div>
              )}

              {isFull && (
                <div className="p-3 rounded-xl border border-warning/30 bg-warning/10 text-warning text-xs space-y-1">
                  <p className="font-semibold">{t('p2p.acceptInvite.fullTitle', 'Room is already full')}</p>
                  <p className="text-muted-foreground text-[11px]">
                    {t('p2p.acceptInvite.fullDetail', 'Another player already joined this 1v1 duel.')}
                  </p>
                </div>
              )}

              {isCreator && (
                <div className="p-3 rounded-xl border border-indigo-500/30 bg-indigo-500/10 text-indigo-400 text-xs space-y-2">
                  <p className="font-semibold">{t('p2p.acceptInvite.creatorTitle', 'You created this challenge!')}</p>
                  <p className="text-muted-foreground text-[11px]">
                    {t('p2p.acceptInvite.creatorDetail', 'Share this code with your friend or re-enter the arena to wait for them.')}
                  </p>
                  <div className="flex gap-2 pt-1">
                    <Button size="sm" variant="outline" onClick={handleCopyLink} className="h-8 text-xs gap-1.5">
                      {copiedLink ? <Check className="w-3.5 h-3.5 text-success" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>{copiedLink ? t('p2p.challenge.copied', 'Copied') : t('p2p.acceptInvite.copyShareLink', 'Copy Share Link')}</span>
                    </Button>
                    <Button
                      size="sm"
                      onClick={() => {
                        onClose()
                        navigate(`/games/match/${invite.match_id}`)
                      }}
                      className="h-8 text-xs font-semibold"
                    >
                      {t('p2p.acceptInvite.enterRoom', 'Enter Room')}
                    </Button>
                  </div>
                </div>
              )}

              {invite.competition_mode === 'earn' && isJoinable && (
                <p className="px-1 text-[11px] leading-relaxed text-muted-foreground">
                  {t('p2p.acceptInvite.acceptReserveNotice', {
                    amount: formatCurrency(invite.stake_amount),
                    defaultValue: `Accepting reserves ${formatCurrency(invite.stake_amount)} from your available balance. The stake settles only when the match ends; draws and cancellations are refunded in full.`,
                  })}
                </p>
              )}

              {/* Ticking countdown for active joinable room */}
              {isJoinable && displaySecondsLeft > 0 && (
                <div className="flex items-center justify-between text-xs text-muted-foreground px-1">
                  <span>{t('p2p.acceptInvite.invitationActive', 'Invitation active')}</span>
                  <span className="font-mono font-semibold text-warning">
                    {t('p2p.challenge.expiresIn', {
                      time: formatCountdown(displaySecondsLeft),
                      defaultValue: `Expires in ${formatCountdown(displaySecondsLeft)}`,
                    })}
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
                          <span>{t('p2p.acceptInvite.joining', 'Joining Match…')}</span>
                        </>
                      ) : (
                        <>
                          <Swords className="w-4 h-4" />
                          <span>
                            {invite.competition_mode === 'earn'
                              ? t('p2p.acceptInvite.acceptAndStake', {
                                  amount: formatCurrency(invite.stake_amount),
                                  defaultValue: `Accept & Stake ${formatCurrency(invite.stake_amount)}`,
                                })
                              : t('p2p.acceptInvite.acceptChallengeEmoji', 'Accept Challenge ⚔️')}
                          </span>
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
                      <span>{t('p2p.acceptInvite.signInToAccept', 'Sign In to Accept Challenge')}</span>
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
                    <span>{t('p2p.acceptInvite.createNewChallenge', 'Create a New Challenge')}</span>
                  </Button>
                ) : null}

                <Button variant="ghost" size="sm" onClick={onClose} className="text-xs text-muted-foreground">
                  {t('common.close', 'Close')}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
