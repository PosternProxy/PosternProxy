import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User } from '@/types'
import * as authApi from '@/api/auth'

// The access token is held in memory only — never in localStorage or
// sessionStorage. On page reload the token is lost; tryRestore() recovers
// it by exchanging the HttpOnly refresh cookie via /auth/refresh.
export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const user = ref<User | null>(null)

  // restorePromise deduplicates concurrent tryRestore() calls (e.g. when the
  // router guard fires for the initial nav and the first API request fires
  // from a component mount at roughly the same time).
  let restorePromise: Promise<boolean> | null = null

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  function setToken(t: string) {
    token.value = t
  }

  async function login(email: string, password: string) {
    const response = await authApi.login(email, password)
    setToken(response.access_token)
    user.value = response.user
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      user.value = await authApi.getMe()
    } catch {
      logout()
    }
  }

  // tryRestore exchanges the HttpOnly refresh cookie for a fresh access
  // token. Returns true if a session was restored, false otherwise. Safe to
  // call when already authenticated (resolves true immediately).
  async function tryRestore(): Promise<boolean> {
    if (token.value) return true
    if (restorePromise) return restorePromise
    restorePromise = (async () => {
      try {
        const { access_token } = await authApi.refresh()
        setToken(access_token)
        return true
      } catch {
        return false
      } finally {
        restorePromise = null
      }
    })()
    return restorePromise
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // ignore
    }
    token.value = null
    user.value = null
  }

  return { token, user, isAuthenticated, isAdmin, setToken, login, fetchUser, tryRestore, logout }
})
