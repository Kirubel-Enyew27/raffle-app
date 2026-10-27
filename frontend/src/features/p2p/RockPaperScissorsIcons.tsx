import type { ReactNode } from 'react'

import { RPS_CHOICES, RPS_CHOICE_MAP, type RpsChoice } from './rockPaperScissors'

/**
 * Hand-drawn move icons for Rock-Paper-Scissors.
 *
 * All three share one 24x24 grid, a 1.75 stroke, round caps/joins and a single
 * 12% body fill on the closed silhouettes, so they read as one set at any size.
 * They are decorative: every place they appear is labelled with text, which is
 * what screen readers announce, so the SVGs are hidden from the a11y tree.
 */
export type RpsIconProps = { className?: string }

const DEFAULT_ICON_SIZE = 'h-8 w-8'

function iconClassName(className?: string) {
  return ['shrink-0', className ?? DEFAULT_ICON_SIZE].join(' ')
}

function SvgFrame({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={iconClassName(className)}
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {children}
    </svg>
  )
}

/** A faceted stone: heptagonal silhouette with three facet ridges. */
export function RockIcon({ className }: RpsIconProps) {
  return (
    <SvgFrame className={className}>
      <path d="M12 3 18.4 6.6 21 13.4 16.4 20H8.4L3 14 5.3 7.2Z" fill="currentColor" fillOpacity="0.12" />
      <path d="M5.3 7.2 12 10.4 18.4 6.6" />
      <path d="M12 10.4 8.4 20" />
      <path d="M12 10.4 16.4 20" />
    </SvgFrame>
  )
}

/** A leaf of paper with a folded corner and two ruled lines. */
export function PaperIcon({ className }: RpsIconProps) {
  return (
    <SvgFrame className={className}>
      <path d="M5.75 3.5h8.05L18.25 8.05V20.5H5.75Z" fill="currentColor" fillOpacity="0.12" />
      <path d="M13.8 3.5v4.55h4.45" />
      <path d="M9 11.5h6.25" />
      <path d="M9 15h4.5" />
    </SvgFrame>
  )
}

/** Two blades crossing at a pivot with two finger loops. */
export function ScissorsIcon({ className }: RpsIconProps) {
  return (
    <SvgFrame className={className}>
      <circle cx="7.5" cy="17" r="2.75" />
      <circle cx="16.5" cy="17" r="2.75" />
      <path d="M9.6 15.1 18.2 3.4" />
      <path d="M14.4 15.1 5.8 3.4" />
      <circle cx="12" cy="11.85" r="0.95" fill="currentColor" stroke="none" />
    </SvgFrame>
  )
}

export function RpsChoiceIcon({ choice, className }: RpsIconProps & { choice: RpsChoice }) {
  switch (choice) {
    case 'rock':
      return <RockIcon className={className} />
    case 'paper':
      return <PaperIcon className={className} />
    default:
      return <ScissorsIcon className={className} />
  }
}

/** Compact all-three badge used wherever the game itself is illustrated. */
export function RpsChoiceTrio({ className }: { className?: string }) {
  const size = className ?? 'h-4 w-4'
  return (
    <span className="inline-flex items-center gap-0.5">
      {RPS_CHOICES.map((choice) => (
        <RpsChoiceIcon
          key={choice.id}
          choice={choice.id}
          className={`${size} ${RPS_CHOICE_MAP[choice.id].accentClass}`}
        />
      ))}
    </span>
  )
}
