import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getTLSStatus } from '@/api/health'
import type { TLSProbeStatus } from '@/types'

export function useTLSStatus(resourceType: string) {
  const statuses = ref<TLSProbeStatus[]>([])
  const loadError = ref('')
  let timer: ReturnType<typeof setInterval> | undefined
  const byID = computed(
    () =>
      new Map(
        statuses.value
          .filter((status) => (status.resource_type ?? 'proxy_host') === resourceType)
          .map((status) => [status.host_id, status])
      )
  )
  async function refresh() {
    try {
      statuses.value = await getTLSStatus()
      loadError.value = ''
    } catch {
      loadError.value = 'Certificate checks are unavailable. Check controller and Caddy status.'
    }
  }
  onMounted(() => {
    void refresh()
    timer = setInterval(refresh, 30_000)
  })
  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })
  return { tlsFor: (id: number) => byID.value.get(id), tlsError: loadError, refreshTLS: refresh }
}
