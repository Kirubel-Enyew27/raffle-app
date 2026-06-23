import axios, { type AxiosError } from 'axios'

export const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
})

// Attach JWT token from storage on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// Normalise error shape; redirect to /login on 401
api.interceptors.response.use(
  (res) => res,
  (err: AxiosError<{ error?: { message?: string }; message?: string }>) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
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
