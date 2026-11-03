import kwikWordmark from '@/assets/kwik-wordmark.png'

interface KwikWordmarkProps {
  className?: string
  alt?: string
}

/**
 * The KWIK wordmark (white lettering on a transparent background), derived from
 * the supplied brand asset. Used by the auth screens and the application chrome
 * so the logo is defined in one place.
 */
export function KwikWordmark({ className = 'h-8 w-auto', alt = 'KWIK' }: KwikWordmarkProps) {
  return (
    <img
      src={kwikWordmark}
      alt={alt}
      className={`${className} select-none`}
      draggable={false}
    />
  )
}
