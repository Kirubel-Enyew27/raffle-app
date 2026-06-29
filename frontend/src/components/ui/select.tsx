import * as React from 'react'
import { cn } from '@/lib/utils'
import { ChevronDown } from 'lucide-react'

interface SelectProps extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'onChange' | 'value'> {
  value?: string
  onChange?: (e: React.ChangeEvent<HTMLSelectElement>) => void
  placeholder?: string
}

const Select = React.forwardRef<HTMLDivElement, SelectProps>(
  ({ className, children, value, onChange, placeholder, disabled, ...props }, ref) => {
    const [open, setOpen] = React.useState(false)
    const containerRef = React.useRef<HTMLDivElement>(null)
    const triggerRef = React.useRef<HTMLButtonElement>(null)
    const listRef = React.useRef<HTMLUListElement>(null)
    const [selectedLabel, setSelectedLabel] = React.useState('')

    // Merge refs
    React.useImperativeHandle(ref, () => containerRef.current!)

    // Build options array from children
    const options = React.useMemo(() => {
      const opts: { value: string; label: string }[] = []
      React.Children.forEach(children, (child) => {
        if (React.isValidElement<React.OptionHTMLAttributes<HTMLOptionElement>>(child) && child.type === 'option') {
          opts.push({
            value: child.props.value as string ?? '',
            label: child.props.children as string ?? child.props.value as string ?? '',
          })
        }
      })
      return opts
    }, [children])

    // Update selected label when value changes
    React.useEffect(() => {
      const opt = options.find(o => o.value === value)
      setSelectedLabel(opt?.label ?? placeholder ?? '')
    }, [value, options, placeholder])

    // Close on click outside
    React.useEffect(() => {
      if (!open) return
      const handler = (e: MouseEvent) => {
        if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
          setOpen(false)
        }
      }
      document.addEventListener('mousedown', handler)
      return () => document.removeEventListener('mousedown', handler)
    }, [open])

    // Close on Escape
    React.useEffect(() => {
      if (!open) return
      const handler = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          setOpen(false)
          triggerRef.current?.focus()
        }
      }
      document.addEventListener('keydown', handler)
      return () => document.removeEventListener('keydown', handler)
    }, [open])

    const handleSelect = (optValue: string) => {
      if (onChange) {
        const nativeEvent = new Event('change', { bubbles: true })
        const fakeTarget = { value: optValue } as HTMLSelectElement
        Object.defineProperty(nativeEvent, 'target', { value: fakeTarget })
        onChange(nativeEvent as unknown as React.ChangeEvent<HTMLSelectElement>)
      }
      setOpen(false)
      triggerRef.current?.focus()
    }

    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault()
        setOpen(prev => !prev)
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setOpen(true)
      }
      if (e.key === 'Escape') {
        setOpen(false)
      }
    }

    return (
      <div ref={containerRef} className={cn('relative', className)} {...props}>
        <button
          ref={triggerRef}
          type="button"
          role="combobox"
          aria-expanded={open}
          aria-haspopup="listbox"
          disabled={disabled}
          onMouseDown={(e) => { e.preventDefault(); if (!disabled) setOpen(prev => !prev) }}
          onKeyDown={handleKeyDown}
          className={cn(
            'flex h-9 w-full cursor-pointer items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm text-foreground shadow-sm transition-colors',
            'hover:bg-accent hover:text-accent-foreground',
            'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
            'disabled:cursor-not-allowed disabled:opacity-50',
            !selectedLabel && placeholder && 'text-muted-foreground',
          )}
        >
          <span className="truncate">{selectedLabel || placeholder || value}</span>
          <ChevronDown
            className={cn(
              'ml-2 h-4 w-4 shrink-0 text-muted-foreground transition-transform',
              open && 'rotate-180',
            )}
          />
        </button>

        {open && (
          <ul
            ref={listRef}
            role="listbox"
            tabIndex={-1}
            className={cn(
              'absolute z-50 mt-1 w-full min-w-[8rem] overflow-auto rounded-md border border-border bg-popover p-1 shadow-md',
            )}
            style={{ maxHeight: '15rem' }}
          >
            {options.length === 0 && (
              <li className="px-2 py-1.5 text-sm text-muted-foreground">No options</li>
            )}
            {options.map((opt) => (
              <li
                key={opt.value}
                role="option"
                aria-selected={opt.value === value}
                onMouseDown={(e) => { e.preventDefault(); handleSelect(opt.value) }}
                onKeyDown={(e) => { if (e.key === 'Enter') handleSelect(opt.value) }}
                tabIndex={opt.value === value ? 0 : -1}
                className={cn(
                  'relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none',
                  'hover:bg-accent hover:text-accent-foreground',
                  opt.value === value && 'bg-accent text-accent-foreground font-medium',
                )}
              >
                {opt.label}
              </li>
            ))}
          </ul>
        )}
      </div>
    )
  },
)
Select.displayName = 'Select'

export { Select }
