import React, { createContext, useContext, useEffect, useRef } from 'react'
import { useAuth } from './AuthContext'
import { useToast } from './ToastContext'
import { useQueryClient } from '@tanstack/react-query'

const RealtimeContext = createContext<null>(null)

export function RealtimeProvider({ children }: { children: React.ReactNode }) {
  const { token, user } = useAuth()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  const eventSourceRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!token) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
        eventSourceRef.current = null
      }
      return
    }

    const baseURL = import.meta.env.VITE_API_URL || '/api/v1'
    const absoluteURL = baseURL.startsWith('http')
      ? `${baseURL}/realtime/stream?token=${encodeURIComponent(token)}`
      : `${window.location.origin}${baseURL}/realtime/stream?token=${encodeURIComponent(token)}`

    console.log('Connecting to real-time events at:', absoluteURL.replace(token, 'REDACTED'))
    const eventSource = new EventSource(absoluteURL)
    eventSourceRef.current = eventSource

    eventSource.addEventListener('connected', () => {
      console.log('Connected to real-time update stream')
    })

    eventSource.addEventListener('message', (event) => {
      try {
        const ev = JSON.parse(event.data)
        console.log('Real-time event:', ev)

        switch (ev.type) {
          case 'wallet_update':
            queryClient.invalidateQueries({ queryKey: ['wallet'] })
            toast({
              title: 'Wallet Balance Updated',
              description: `Your balance is now ${ev.payload.balance} ${ev.payload.currency || 'ETB'}.`,
              type: 'success',
            })
            break

          case 'transaction_update':
            queryClient.invalidateQueries({ queryKey: ['transactions'] })
            if (ev.payload.type === 'deposit' && ev.payload.status === 'completed') {
              toast({
                title: 'Deposit Successful',
                description: `Successfully deposited ${ev.payload.amount} ETB.`,
                type: 'success',
              })
            } else if (ev.payload.type === 'withdrawal') {
              if (ev.payload.status === 'pending') {
                toast({
                  title: 'Withdrawal Requested',
                  description: `Your request for ${ev.payload.amount} ETB is pending admin approval.`,
                  type: 'info',
                })
              } else if (ev.payload.status === 'completed') {
                toast({
                  title: 'Withdrawal Processed',
                  description: `Your withdrawal of ${ev.payload.amount} ETB was approved and completed.`,
                  type: 'success',
                })
              } else if (ev.payload.status === 'rejected') {
                toast({
                  title: 'Withdrawal Rejected',
                  description: `Your withdrawal request of ${ev.payload.amount} ETB was rejected.`,
                  type: 'error',
                })
              }
            }
            break

          case 'ticket_purchase':
            // Invalidate detailed and list queries for the updated raffle
            queryClient.invalidateQueries({ queryKey: ['raffle', ev.payload.raffle_id] })
            queryClient.invalidateQueries({ queryKey: ['raffles'] })
            break

          case 'withdrawal_request':
            if (user?.role === 'admin') {
              queryClient.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
              toast({
                title: 'Withdrawal Action Required',
                description: `A user has requested a withdrawal of ${ev.payload.amount} ETB.`,
                type: 'warning',
              })
            }
            break

          case 'withdrawal_status':
            queryClient.invalidateQueries({ queryKey: ['wallet'] })
            queryClient.invalidateQueries({ queryKey: ['transactions'] })
            queryClient.invalidateQueries({ queryKey: ['admin-pending-withdrawals'] })
            break

          case 'draw_completed':
            queryClient.invalidateQueries({ queryKey: ['raffle', ev.payload.raffle_id] })
            queryClient.invalidateQueries({ queryKey: ['raffles'] })
            break

          case 'winner_announcement':
            queryClient.invalidateQueries({ queryKey: ['winners'] })
            queryClient.invalidateQueries({ queryKey: ['admin-winners'] })
            queryClient.invalidateQueries({ queryKey: ['my-winners'] })
            break

          case 'winner_notification':
            toast({
              title: '🎉 Congratulations!',
              description: `You are the winner of the raffle! A prize of ${ev.payload.prize_amount} ETB has been sent to your wallet!`,
              type: 'success',
            })
            // Force refresh data
            queryClient.invalidateQueries({ queryKey: ['wallet'] })
            queryClient.invalidateQueries({ queryKey: ['transactions'] })
            queryClient.invalidateQueries({ queryKey: ['my-winners'] })
            break

          default:
            break
        }
      } catch (err) {
        console.error('Failed to handle real-time message:', err)
      }
    })

    eventSource.onerror = (err) => {
      console.warn('Real-time connection error/reconnect attempt:', err)
    }

    return () => {
      console.log('Closing real-time connection')
      eventSource.close()
      if (eventSourceRef.current === eventSource) {
        eventSourceRef.current = null
      }
    }
  }, [token, user?.role, queryClient, toast])

  return <RealtimeContext.Provider value={null}>{children}</RealtimeContext.Provider>
}

export function useRealtime() {
  return useContext(RealtimeContext)
}
