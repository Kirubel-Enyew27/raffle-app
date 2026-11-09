import React from 'react'
import { Volume2, VolumeX } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'

interface SoundToggleProps {
  isMuted: boolean
  onToggle: () => void
  className?: string
}

export const SoundToggle: React.FC<SoundToggleProps> = ({ isMuted, onToggle, className = '' }) => {
  const { t } = useTranslation()

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={onToggle}
      className={`h-7 px-2.5 gap-1.5 text-xs font-medium border-border transition-colors ${
        isMuted
          ? 'text-muted-foreground hover:text-foreground'
          : 'text-primary hover:text-primary/80 bg-primary/5'
      } ${className}`}
      aria-label={isMuted ? t('p2p.sound.unmuteAria', 'Unmute game audio effects') : t('p2p.sound.muteAria', 'Mute game audio effects')}
      title={isMuted ? t('p2p.sound.unmuteTitle', 'Audio muted — click to enable sound effects') : t('p2p.sound.muteTitle', 'Audio active — click to mute sound effects')}
    >
      {isMuted ? (
        <>
          <VolumeX className="w-3.5 h-3.5 text-muted-foreground" />
          <span className="hidden sm:inline">{t('p2p.sound.muted', 'Muted')}</span>
        </>
      ) : (
        <>
          <Volume2 className="w-3.5 h-3.5 text-primary" />
          <span className="hidden sm:inline">{t('p2p.sound.soundOn', 'Sound On')}</span>
        </>
      )}
    </Button>
  )
}
