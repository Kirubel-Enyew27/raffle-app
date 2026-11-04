import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

type Theme = 'light' | 'dark' | 'system'
type ResolvedTheme = 'light' | 'dark'

interface ThemeContextValue {
  theme: Theme
  /** The theme currently applied to the document (`system` is resolved). */
  resolvedTheme: ResolvedTheme
  setTheme: (t: Theme) => void
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export function ThemeProvider({ children }: { children: ReactNode }) {
  // KWIK is a dark-first product: until someone picks a theme, use the brand
  // dark surface. The toggle still lets them choose light or system later.
  const [theme, setThemeState] = useState<Theme>(
    () => (localStorage.getItem('theme') as Theme) ?? 'dark',
  )
  const [systemPrefersDark, setSystemPrefersDark] = useState(
    () => window.matchMedia('(prefers-color-scheme: dark)').matches,
  )

  const resolvedTheme: ResolvedTheme =
    theme === 'dark' || (theme === 'system' && systemPrefersDark) ? 'dark' : 'light'

  // Keep `system` reactive so branded artwork follows live OS theme changes.
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handleChange = (event: MediaQueryListEvent) => setSystemPrefersDark(event.matches)

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handleChange)
      return () => mediaQuery.removeEventListener('change', handleChange)
    }
    // Older browsers (Safari < 14)
    mediaQuery.addListener(handleChange)
    return () => mediaQuery.removeListener(handleChange)
  }, [])

  useEffect(() => {
    const root = document.documentElement
    const isDark = resolvedTheme === 'dark'
    root.classList.toggle('dark', isDark)

    // Keep mobile/TMA browser chrome and the tab icon in step with the page.
    const themeColor = document.querySelector('meta[name="theme-color"]')
    if (themeColor) themeColor.setAttribute('content', isDark ? '#0a0a0b' : '#f7f8fa')

    document.querySelectorAll<HTMLLinkElement>('link[data-kwik-favicon]').forEach((link) => {
      const size = link.dataset.size
      if (size) link.href = `/brand/favicon-${resolvedTheme}-${size}.png`
    })

    localStorage.setItem('theme', theme)
  }, [theme, resolvedTheme])

  const setTheme = (t: Theme) => setThemeState(t)

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
