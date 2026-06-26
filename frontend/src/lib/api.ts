import axios, { type AxiosError } from 'axios'

// In production (Vercel), VITE_API_URL points to the Koyeb backend.
// In development, Vite's proxy handles /api requests to localhost:8080.
const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

// Attach JWT token from storage on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// Normalise error shape; redirect to /login on 401 (except for login requests)
api.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ error?: { message?: string }; message?: string }>) => {
    if (err.response?.status === 401 && !err.config?.url?.includes('/auth/login')) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      // Redirect immediately and suppress the error from reaching components.
      // The never-settling promise prevents React Query from updating error state
      // before the page navigation takes over.
      window.location.href = '/login'
      return new Promise<never>(() => {})
    }
    const message =
      err.response?.data?.error?.message ??
      err.response?.data?.message ??
      err.message ??
      'An unexpected error occurred'
    return Promise.reject(new Error(message))
  },
)

export type ApiError = Error
