import { useTheme } from '@/contexts/ThemeContext'
import { cn } from '@/lib/utils'

type KwikWordmarkVariant = 'full' | 'compact'
type LogoTheme = 'light' | 'dark'

interface KwikWordmarkProps {
  /** `full` for auth/splash, `compact` for application chrome. */
  variant?: KwikWordmarkVariant
  className?: string
  alt?: string
}

const SIZE_BY_VARIANT: Record<KwikWordmarkVariant, string> = {
  full: 'h-16 w-48 sm:h-20 sm:w-60',
  compact: 'h-10 w-[120px]',
}

const SRC_BY_THEME: Record<LogoTheme, string> = {
  dark: '/brand/kwik-logo-for-dark-theme.png',
  light: '/brand/kwik-logo-for-light-theme.png',
}

/**
 * The official KWIK game wordmark.
 *
 * Both supplied theme assets are mounted so a theme switch is instant. The
 * files themselves are used unmodified; the fixed 3:1 frame only trims the
 * empty margins around the artwork.
 */
export function KwikWordmark({
  variant = 'full',
  className,
  alt = 'KWIK',
}: KwikWordmarkProps) {
  const { resolvedTheme } = useTheme()

  return (
    <span
      role="img"
      aria-label={alt}
      className="inline-grid shrink-0 leading-none"
    >
      {(Object.keys(SRC_BY_THEME) as LogoTheme[]).map((theme) => (
        <img
          key={theme}
          src={SRC_BY_THEME[theme]}
          alt=""
          aria-hidden="true"
          width={1024}
          height={1024}
          className={cn(
            'col-start-1 row-start-1 block max-w-full select-none object-cover',
            SIZE_BY_VARIANT[variant],
            resolvedTheme === theme ? 'opacity-100' : 'opacity-0',
            className,
          )}
          draggable={false}
          decoding="async"
        />
      ))}
    </span>
  )
}
