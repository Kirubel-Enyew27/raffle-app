import React, { createContext, useContext, useEffect, useRef, useState } from 'react'
import { useAuth } from './AuthContext'
import { useToast } from './ToastContext'
import { useQueryClient } from '@tanstack/react-query'
import { sseClient } from '@/lib/sseClient'
import type { SSEEvent, SSEStatus } from '@/lib/sseClient'

// ─── Context type ─────────────────────────────────────────────────────────────

interface RealtimeContextValue {
  connectionStatus: SSEStatus
}

const RealtimeContext = createContext<RealtimeContextValue>({ connectionStatus: 'closed' })

// ─── Provider ─────────────────────────────────────────────────────────────────

export function RealtimeProvider({ children }: { children: React.ReactNode }) {
  const { token, user } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const [connectionStatus, setConnectionStatus] = useState<SSEStatus>('closed')
  const lastStatusRef = useRef<SSEStatus>('closed')

  useEffect(() => {
    if (!token) {
      setConnectionStatus('closed')
      lastStatusRef.current = 'closed'
      return
    }

    // sseClient internally handles AUTH_TOKEN_REFRESHED_EVENT — it will
    // reconnect with the new token and migrate our subscriber automatically.
    // We just need to provide the current token here.

    const CHANNEL = 'global'

    console.log('Connecting to real-time events (sseClient)')
    setConnectionStatus('connecting')
    lastStatusRef.current = 'connecting'

    const unsub = sseClient.subscribe(CHANNEL, token, (ev: SSEEvent) => {
      handleEvent(ev)
    })

    const interval = setInterval(() => {
      const s = sseClient.getStatus(CHANNEL)
      if (s !== lastStatusRef.current) {
        lastStatusRef.current = s
        setConnectionStatus(s)

        if (s === 'live') {
          console.log('Connected to real-time update stream')
        } else if (s === 'reconnecting') {
          console.warn('Real-time connection lost — reconnecting with backoff')
        } else if (s === 'failed') {
          console.error('Real-time connection failed after max retries')
        }
      }
    }, 200)

    return () => {
      clearInterval(interval)
      unsub()
      setConnectionStatus('closed')
      lastStatusRef.current = 'closed'
    }

    function handleEvent(ev: SSEEvent) {
      const raw = ev as {
        type: string
        payload: Record<string, unknown>
      }
      console.log('Real-time event:', raw)

      switch (raw.type) {
        case 'wallet_update': {
          queryClient.invalidateQueries({ queryKey: ['wallet'] })
          toast({
            title: 'Wallet Balance Updated',
            description: `Your balance is now ${(raw.payload as { balance: unknown }).balance} ${(raw.payload as { currency?: string }).currency ?? 'ETB'}.`,
            type: 'success',
          })
          break
        }

        case 'transaction_update': {
          queryClient.invalidateQueries({ queryKey: ['transactions'] })
          const payload = raw.payload as {
            type?: string
            status?: string
            amount?: number
          }
          if (payload.type === 'ticket_purchase') break

          const txType =
            payload.type === 'deposit' || payload.type === 'sms_deposit' ? 'deposit' : 'withdraw'

          if (txType === 'deposit' && payload.status === 'completed') {
            toast({
              title: 'Deposit Successful',
              description: `Successfully deposited ${payload.amount} ETB.`,
              type: 'success',
            })
          } else if (txType === 'withdraw') {
            if (payload.status === 'pending') {
              toast({
                title: 'Withdrawal Requested',
                description: `Your request for ${payload.amount} ETB is pending admin approval.`,
                type: 'info',
              })
            } else if (payload.status === 'completed') {
              toast({
                title: 'Withdrawal Processed',
                description: `Your withdrawal of ${payload.amount} ETB was approved and completed.`,
                type: 'success',
              })
            } else if (payload.status === 'rejected') {
              toast({
                title: 'Withdrawal Rejected',
                description: `Your withdrawal request of ${payload.amount} ETB was rejected.`,
                type: 'error',
              })
            }
          }
          break
        }

        case 'ticket_purchase': {
          const payload = raw.payload as { raffle_id?: string }
          queryClient.invalidateQueries({ queryKey: ['raffle', payload.raffle_id] })
          queryClient.invalidateQueries({ queryKey: ['raffles'] })
          break
        }

        case 'withdrawal_request': {
          if (user?.role === 'admin') {
            queryClient.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
            toast({
              title: 'Withdrawal Action Required',
              description: `A user has requested a withdrawal of ${(raw.payload as { amount?: number }).amount} ETB.`,
              type: 'warning',
            })
          }
          break
        }

        case 'withdrawal_status': {
          queryClient.invalidateQueries({ queryKey: ['wallet'] })
          queryClient.invalidateQueries({ queryKey: ['transactions'] })
          queryClient.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
          break
        }

        case 'draw_completed': {
          const payload = raw.payload as { raffle_id?: string }
          queryClient.invalidateQueries({ queryKey: ['raffle', payload.raffle_id] })
          queryClient.invalidateQueries({ queryKey: ['raffles'] })
          break
        }

        case 'winner_announcement': {
          queryClient.invalidateQueries({ queryKey: ['winners'] })
          queryClient.invalidateQueries({ queryKey: ['admin-winners'] })
          queryClient.invalidateQueries({ queryKey: ['my-winners'] })
          break
        }

        case 'winner_notification': {
          const payload = raw.payload as { prize_amount?: number }
          toast({
            title: '🎉 Congratulations!',
            description: `You are the winner of the raffle! A prize of ${payload.prize_amount} ETB has been sent to your wallet!`,
            type: 'success',
          })
          queryClient.invalidateQueries({ queryKey: ['wallet'] })
          queryClient.invalidateQueries({ queryKey: ['transactions'] })
          queryClient.invalidateQueries({ queryKey: ['my-winners'] })
          break
        }

        default:
          break
      }
    }
  }, [token, user?.role, queryClient, toast])

  return (
    <RealtimeContext.Provider value={{ connectionStatus }}>
      {children}
    </RealtimeContext.Provider>
  )
}

// ─── Consumer hooks ───────────────────────────────────────────────────────────

export function useRealtime() {
  return useContext(RealtimeContext)
}

/** Returns the current SSE connection status for the global realtime channel. */
export function useRealtimeStatus(): SSEStatus {
  return useContext(RealtimeContext).connectionStatus
}
