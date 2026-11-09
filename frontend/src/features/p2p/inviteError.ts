import type { TFunction } from 'i18next'

export interface InviteErrorInfo {
  title: string
  description: string
  canRetry: boolean
  isExpired: boolean
}

export function parseInviteError(err: unknown, cleanCode: string, t?: TFunction): InviteErrorInfo {
  if (err && typeof err === 'object') {
    const e = err as {
      status?: number
      code?: string
      message?: string
      response?: { status?: number; data?: { code?: string; message?: string } }
    }
    const status = e.status || e.response?.status
    const code = e.code || e.response?.data?.code
    const message = e.response?.data?.message || e.message || ''

    if (status === 404 || code === 'NOT_FOUND' || message.toLowerCase().includes('not found')) {
      return {
        title: t ? t('p2p.acceptInvite.challengeNotFound', 'Challenge Not Found') : 'Challenge Not Found',
        description: t
          ? t('p2p.acceptInvite.verifyCode', {
              code: cleanCode,
              defaultValue: `We couldn't find an active challenge with code "${cleanCode}". Please verify the code with your friend.`,
            })
          : `We couldn't find an active challenge with code "${cleanCode}". Please verify the code with your friend.`,
        canRetry: false,
        isExpired: false,
      }
    }

    if (status === 410 || code === 'INVITATION_EXPIRED' || message.toLowerCase().includes('expired')) {
      return {
        title: t ? t('p2p.acceptInvite.challengeExpired', 'Challenge Expired') : 'Challenge Expired',
        description: t
          ? t('p2p.acceptInvite.expiredDesc', 'This challenge invitation has expired (challenges are valid for 5 minutes).')
          : 'This challenge invitation has expired (challenges are valid for 5 minutes).',
        canRetry: false,
        isExpired: true,
      }
    }

    if (status === 403 || code === 'FORBIDDEN') {
      return {
        title: t ? t('p2p.acceptInvite.accessRestricted', 'Access Restricted') : 'Access Restricted',
        description: t
          ? t('p2p.acceptInvite.forbiddenDesc', 'You do not have permission to access this challenge.')
          : 'You do not have permission to access this challenge.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'CANNOT_JOIN_OWN_MATCH' || message.toLowerCase().includes('own match')) {
      return {
        title: t ? t('p2p.acceptInvite.ownChallenge', 'Your Challenge') : 'Your Challenge',
        description: t
          ? t('p2p.acceptInvite.ownMatchDesc', 'You cannot join your own challenge as an opponent.')
          : 'You cannot join your own challenge as an opponent.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'MATCH_FULL' || message.toLowerCase().includes('full')) {
      return {
        title: t ? t('p2p.acceptInvite.challengeFull', 'Challenge Full') : 'Challenge Full',
        description: t
          ? t('p2p.acceptInvite.matchFullDesc', 'This challenge already has the maximum number of players.')
          : 'This challenge already has the maximum number of players.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'MATCH_NOT_JOINABLE' || message.toLowerCase().includes('not joinable')) {
      return {
        title: t ? t('p2p.acceptInvite.challengeUnavailable', 'Challenge Unavailable') : 'Challenge Unavailable',
        description: t
          ? t('p2p.acceptInvite.notJoinableDesc', 'This challenge is no longer open for joining.')
          : 'This challenge is no longer open for joining.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (
      status === 0 ||
      status === undefined ||
      message.toLowerCase().includes('network') ||
      message.toLowerCase().includes('connect')
    ) {
      return {
        title: t ? t('p2p.acceptInvite.connectionError', 'Connection Error') : 'Connection Error',
        description: t
          ? t('p2p.acceptInvite.networkDesc', 'Unable to reach the server. Please check your connection and try again.')
          : 'Unable to reach the server. Please check your connection and try again.',
        canRetry: true,
        isExpired: false,
      }
    }

    return {
      title: t ? t('p2p.acceptInvite.challengeUnavailable', 'Unable to Load Challenge') : 'Unable to Load Challenge',
      description: message || (t ? t('errors.somethingWentWrong', 'An unexpected error occurred while loading this challenge.') : 'An unexpected error occurred while loading this challenge.'),
      canRetry: true,
      isExpired: false,
    }
  }

  return {
    title: t ? t('p2p.acceptInvite.challengeUnavailable', 'Unable to Load Challenge') : 'Unable to Load Challenge',
    description: t ? t('errors.somethingWentWrong', 'An unexpected error occurred while loading this challenge.') : 'An unexpected error occurred while loading this challenge.',
    canRetry: true,
    isExpired: false,
  }
}
