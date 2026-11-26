<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { listConfigurationStates, type ConfigurationState } from '@/api/configuration'
import { resyncServer } from '@/api/servers'
import { useAuthStore } from '@/stores/auth'
import { errorMessage as apiErrorMessage } from '@/utils/errors'

const auth = useAuthStore()
const states = ref<ConfigurationState[]>([])
const error = ref('')
const retrying = ref<number | null>(null)
const outstanding = computed(() => states.value.filter((state) => state.status !== 'applied' || state.applied_revision !== state.desired_revision))
let timer: ReturnType<typeof setInterval> | undefined
let loading = false
let active = true

async function load() {
  if (loading) return
  loading = true
  try {
    const next = await listConfigurationStates()
    if (active) { states.value = next; error.value = '' }
  } catch (e: unknown) {
    if (active) error.value = apiErrorMessage(e, 'Configuration status is unavailable.')
  } finally { loading = false }
}

async function retry(serverID: number) {
  retrying.value = serverID
  try { await resyncServer(serverID) }
  catch (e: unknown) { error.value = apiErrorMessage(e, 'Synchronization failed.') }
  finally { retrying.value = null; await load() }
}

onMounted(() => { void load(); timer = setInterval(() => { void load() }, 5000) })
onUnmounted(() => { active = false; if (timer) clearInterval(timer) })
</script>

<template>
  <section v-if="error || outstanding.length" aria-label="Configuration status" aria-live="polite" class="mb-6 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
    <p v-if="error">{{ error }}</p>
    <div v-for="state in outstanding" :key="state.server_id" class="flex items-center justify-between gap-4 py-1">
      <div>
        <p class="font-medium">Server {{ state.server_id }}: {{ state.status === 'failed' ? 'configuration failed' : state.status === 'applying' ? 'applying saved changes' : 'saved changes waiting to apply' }}</p>
        <p v-if="state.error">{{ state.error }}</p>
      </div>
      <button v-if="auth.isAdmin" type="button" :disabled="retrying !== null || state.status === 'applying'" class="rounded border border-amber-400 px-3 py-1 disabled:opacity-50" @click="retry(state.server_id)">
        {{ retrying === state.server_id ? 'Retrying…' : 'Retry sync' }}
      </button>
    </div>
  </section>
</template>
