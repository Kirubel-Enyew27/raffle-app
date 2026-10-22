import React from 'react'
import { Volume2, VolumeX } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface SoundToggleProps {
  isMuted: boolean
  onToggle: () => void
  className?: string
}

export const SoundToggle: React.FC<SoundToggleProps> = ({ isMuted, onToggle, className = '' }) => {
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
      aria-label={isMuted ? 'Unmute game audio effects' : 'Mute game audio effects'}
      title={isMuted ? 'Audio muted — click to enable sound effects' : 'Audio active — click to mute sound effects'}
    >
      {isMuted ? (
        <>
          <VolumeX className="w-3.5 h-3.5 text-muted-foreground" />
          <span className="hidden sm:inline">Muted</span>
        </>
      ) : (
        <>
          <Volume2 className="w-3.5 h-3.5 text-primary" />
          <span className="hidden sm:inline">Sound On</span>
        </>
      )}
    </Button>
  )
}
