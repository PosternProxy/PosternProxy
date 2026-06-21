import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

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
      if (requestURL.endsWith('/auth/refresh')) {
        auth.clearSession()
        router.push('/login')
        return Promise.reject(error)
      }
      try {
        const { data } = await axios.post('/api/auth/refresh')
        auth.setToken(data.access_token)
        error.config.headers.Authorization = `Bearer ${data.access_token}`
        // Re-read the CSRF cookie before retrying. The refresh response can
        // rotate the csrf_token cookie, and error.config still carries the
        // stale header from the original attempt (audit L-6). Even though the
        // request interceptor would overwrite this on its own, doing it here
        // is explicit and not dependent on interceptor ordering.
        const csrfToken = getCookie('csrf_token')
        if (csrfToken) {
          error.config.headers['X-CSRF-Token'] = csrfToken
        }
        return client.request(error.config)
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
