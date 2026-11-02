import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Loader2 } from 'lucide-react'
import { API_BASE_URL } from '@/lib/api'
import { buildGoogleStartUrl } from '@/features/auth/google'
import { GoogleIcon } from '@/features/auth/GoogleIcon'

interface GoogleSignInButtonProps {
  /** Internal path to return to after a successful sign-in. */
  redirect?: string | null
}

/**
 * Starts the OAuth flow by navigating to the backend entry point. Navigation is
 * guarded by a ref as well as state so a double click cannot fire two
 * authorization attempts.
 */
export function GoogleSignInButton({ redirect }: GoogleSignInButtonProps) {
  const { t } = useTranslation()
  const [redirecting, setRedirecting] = useState(false)
  const starting = useRef(false)

  const handleClick = () => {
    if (starting.current) return
    starting.current = true
    setRedirecting(true)
    window.location.assign(buildGoogleStartUrl(API_BASE_URL, redirect))
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={redirecting}
      aria-busy={redirecting}
      className="inline-flex h-10 w-full items-center justify-center gap-2.5 rounded-md border border-input bg-background px-4 text-sm font-medium shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-70"
    >
      {redirecting ? <Loader2 className="h-[18px] w-[18px] animate-spin" /> : <GoogleIcon />}
      <span>{redirecting ? t('auth.googleRedirecting') : t('auth.continueWithGoogle')}</span>
    </button>
  )
}
