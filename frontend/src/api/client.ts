import axios from 'axios'
import type { InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

type RetryConfig = InternalAxiosRequestConfig & { _authRetried?: boolean }

const client = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

client.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  const csrfToken = getCookie('csrf_token')
  if (csrfToken) {
    config.headers['X-CSRF-Token'] = csrfToken
  }
  return config
})

client.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      const auth = useAuthStore()
      const requestURL = error.config?.url ?? ''
      if (requestURL.endsWith('/auth/login')) return Promise.reject(error)
      const config = error.config as RetryConfig | undefined
      if (requestURL.endsWith('/auth/refresh') || config?._authRetried || !config) {
        auth.clearSession()
        router.push('/login')
        return Promise.reject(error)
      }
      try {
        config._authRetried = true
        const token = await auth.refreshSession()
        config.headers.Authorization = `Bearer ${token}`
        const csrfToken = getCookie('csrf_token')
        if (csrfToken) {
          config.headers['X-CSRF-Token'] = csrfToken
        }
        return client.request(config)
      } catch {
        auth.clearSession()
        router.push('/login')
      }
    }
    return Promise.reject(error)
  }
)

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return match ? (match[2] ?? null) : null
}

export default client
