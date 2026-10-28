import axios, { type AxiosError } from 'axios'
import { authStorage } from '@/lib/authStorage'

// In production (Vercel), VITE_API_URL points to the Render backend.
// In development, Vite's proxy handles /api requests to localhost:8080.
const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

export class ApiError extends Error {
  status?: number
  code?: string
  response?: AxiosError['response']

  constructor(message: string, status?: number, code?: string, response?: AxiosError['response']) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.response = response
  }
}

// Attach JWT token from storage on every request
api.interceptors.request.use((config) => {
  const token = authStorage.getToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// Normalise error shape; redirect to /login on 401 (except for login requests)
api.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ error?: { message?: string; code?: string }; message?: string; code?: string }>) => {
    if (err.response?.status === 401 && !err.config?.url?.includes('/auth/login')) {
      authStorage.clear()
      // Redirect immediately and suppress the error from reaching components.
      // The never-settling promise prevents React Query from updating error state
      // before the page navigation takes over.
      window.location.href = '/login'
      return new Promise<never>(() => {})
    }

    const status = err.response?.status
    const rawData = err.response?.data
    const serverMessage =
      typeof rawData === 'object' && rawData !== null
        ? rawData.message || rawData.error?.message
        : undefined
    const serverCode =
      typeof rawData === 'object' && rawData !== null
        ? rawData.code || rawData.error?.code
        : undefined

    let message = serverMessage
    if (!message) {
      if (status === 404) {
        message = 'The requested resource was not found'
      } else if (err.code === 'ERR_NETWORK' || !err.response) {
        message = 'Unable to connect to the server. Please check your internet connection.'
      } else {
        message = err.message || 'An unexpected error occurred'
      }
    }

    const apiError = new ApiError(message, status, serverCode, err.response)
    return Promise.reject(apiError)
  },
)
