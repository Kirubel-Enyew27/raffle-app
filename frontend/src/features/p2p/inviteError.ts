export interface InviteErrorInfo {
  title: string
  description: string
  canRetry: boolean
  isExpired: boolean
}

export function parseInviteError(err: unknown, cleanCode: string): InviteErrorInfo {
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
        title: 'Challenge Not Found',
        description: `We couldn't find an active challenge with code "${cleanCode}". Please verify the code with your friend.`,
        canRetry: false,
        isExpired: false,
      }
    }

    if (status === 410 || code === 'INVITATION_EXPIRED' || message.toLowerCase().includes('expired')) {
      return {
        title: 'Challenge Expired',
        description: 'This challenge invitation has expired (challenges are valid for 5 minutes).',
        canRetry: false,
        isExpired: true,
      }
    }

    if (status === 403 || code === 'FORBIDDEN') {
      return {
        title: 'Access Restricted',
        description: 'You do not have permission to access this challenge.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'CANNOT_JOIN_OWN_MATCH' || message.toLowerCase().includes('own match')) {
      return {
        title: 'Your Challenge',
        description: 'You cannot join your own challenge as an opponent.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'MATCH_FULL' || message.toLowerCase().includes('full')) {
      return {
        title: 'Challenge Full',
        description: 'This challenge already has the maximum number of players.',
        canRetry: false,
        isExpired: false,
      }
    }

    if (code === 'MATCH_NOT_JOINABLE' || message.toLowerCase().includes('not joinable')) {
      return {
        title: 'Challenge Unavailable',
        description: 'This challenge is no longer open for joining.',
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
        title: 'Connection Error',
        description: 'Unable to reach the server. Please check your connection and try again.',
        canRetry: true,
        isExpired: false,
      }
    }

    return {
      title: 'Unable to Load Challenge',
      description: message || 'An unexpected error occurred while loading this challenge.',
      canRetry: true,
      isExpired: false,
    }
  }

  return {
    title: 'Unable to Load Challenge',
    description: 'An unexpected error occurred while loading this challenge.',
    canRetry: true,
    isExpired: false,
  }
}
