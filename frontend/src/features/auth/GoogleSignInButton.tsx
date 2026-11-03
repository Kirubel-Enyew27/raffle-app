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
      className="inline-flex h-[52px] w-full items-center justify-center gap-3 rounded-xl border border-[#26282e] bg-[#101113] px-4 text-[15px] font-semibold text-white transition-colors hover:bg-[#17181c] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#3d4149] disabled:pointer-events-none disabled:opacity-70"
    >
      {redirecting ? <Loader2 className="h-[18px] w-[18px] animate-spin" /> : <GoogleIcon />}
      <span>{redirecting ? t('auth.googleRedirecting') : t('auth.continueWithGoogle')}</span>
    </button>
  )
}
