import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Server } from '@/types'
import { listServers } from '@/api/servers'

/**
 * Global server list store.
 *
 * Loaded once on app init (or on-demand) and shared across all views that
 * need to present a server selector (proxy hosts, redirection hosts, etc.).
 * The local server (id=1) is always present.
 */
export const useServerStore = defineStore('servers', () => {
  const servers = ref<Server[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  /** True when more than one server exists (local + at least one remote). */
  const hasRemoteServers = computed(() => servers.value.some((s) => !s.is_local))

  /** Options suitable for a <select> dropdown. */
  const serverOptions = computed(() =>
    servers.value.map((s) => ({
      value: s.id,
      label: s.is_local ? `${s.name} (local)` : `${s.name} (${s.hostname})`,
    }))
  )

  async function fetch() {
    if (loading.value) return
    loading.value = true
    try {
      servers.value = await listServers()
      loaded.value = true
    } catch {
      // Non-fatal — host forms fall back to server_id=1
    } finally {
      loading.value = false
    }
  }

  /** Fetch only if not already loaded. */
  async function ensureLoaded() {
    if (!loaded.value) await fetch()
  }

  function getById(id: number): Server | undefined {
    return servers.value.find((s) => s.id === id)
  }

  return { servers, loading, loaded, hasRemoteServers, serverOptions, fetch, ensureLoaded, getById }
})
