import { Sun, Moon, Monitor } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useTheme } from '@/contexts/ThemeContext'
import { Button } from '@/components/ui/button'

type Theme = 'light' | 'dark' | 'system'

export function ThemeSelector() {
  const { theme, setTheme } = useTheme()
  const { t } = useTranslation()

  const options: { value: Theme; icon: React.ReactNode; label: string }[] = [
    { value: 'light',  icon: <Sun     className="h-3.5 w-3.5" />, label: t('theme.light')  },
    { value: 'dark',   icon: <Moon    className="h-3.5 w-3.5" />, label: t('theme.dark')   },
    { value: 'system', icon: <Monitor className="h-3.5 w-3.5" />, label: t('theme.system') },
  ]

  return (
    <div
      role="group"
      aria-label={t('theme.title')}
      className="flex items-center rounded-lg border border-border bg-muted/50 p-0.5 gap-0.5"
    >
      {options.map(({ value, icon, label }) => (
        <Button
          key={value}
          variant="ghost"
          size="sm"
          onClick={() => setTheme(value)}
          aria-label={label}
          aria-pressed={theme === value}
          title={label}
          className={[
            'h-7 gap-1.5 px-2 text-xs font-medium transition-all',
            theme === value
              ? 'bg-background text-foreground shadow-sm hover:bg-background hover:text-foreground'
              : 'text-muted-foreground hover:text-foreground hover:bg-transparent',
          ].join(' ')}
        >
          {icon}
          <span className="hidden sm:inline">{label}</span>
        </Button>
      ))}
    </div>
  )
}
