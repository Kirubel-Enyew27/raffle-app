import React, { createContext, useContext, useState, useCallback } from 'react'
import { CheckCircle, Info, AlertTriangle, XCircle, X } from 'lucide-react'

export interface ToastMessage {
  id: string
  title: string
  description?: string
  type?: 'success' | 'info' | 'warning' | 'error'
}

interface ToastContextValue {
  toast: (msg: Omit<ToastMessage, 'id'>) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastMessage[]>([])

  const toast = useCallback(({ title, description, type = 'info' }: Omit<ToastMessage, 'id'>) => {
    const id = Math.random().toString(36).substring(2, 9)
    setToasts((prev) => [...prev, { id, title, description, type }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 6000)
  }, [])

  const removeToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  const icons = {
    success: <CheckCircle className="h-5 w-5 text-emerald-500 animate-bounce" />,
    info: <Info className="h-5 w-5 text-blue-500 animate-pulse" />,
    warning: <AlertTriangle className="h-5 w-5 text-amber-500" />,
    error: <XCircle className="h-5 w-5 text-rose-500 animate-shake" />,
  }

  const bgColors = {
    success: 'border-emerald-500/20 bg-card/95 text-foreground hover:bg-emerald-500/5 dark:bg-card/95',
    info: 'border-blue-500/20 bg-card/95 text-foreground hover:bg-blue-500/5 dark:bg-card/95',
    warning: 'border-amber-500/20 bg-card/95 text-foreground hover:bg-amber-500/5 dark:bg-card/95',
    error: 'border-rose-500/20 bg-card/95 text-foreground hover:bg-rose-500/5 dark:bg-card/95',
  }

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      {/* Toast Container */}
      <div className="fixed bottom-4 right-4 z-55 flex flex-col gap-2 w-full max-w-sm pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`flex items-start gap-3 p-4 rounded-xl border backdrop-blur-md shadow-xl transition-all duration-300 pointer-events-auto ${
              bgColors[t.type || 'info']
            }`}
            style={{
              animation: 'slideIn 0.35s cubic-bezier(0.16, 1, 0.3, 1) forwards',
            }}
          >
            <div className="shrink-0 mt-0.5">{icons[t.type || 'info']}</div>
            <div className="flex-1 space-y-1">
              <h4 className="font-semibold text-sm tracking-tight text-foreground">{t.title}</h4>
              {t.description && <p className="text-xs text-muted-foreground leading-relaxed">{t.description}</p>}
            </div>
            <button
              onClick={() => removeToast(t.id)}
              className="text-muted-foreground hover:text-foreground p-0.5 rounded-lg transition-colors focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
      <style>{`
        @keyframes slideIn {
          from {
            transform: translateY(1.5rem) scale(0.9);
            opacity: 0;
          }
          to {
            transform: translateY(0) scale(1);
            opacity: 1;
          }
        }
      `}</style>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
