<script setup lang="ts">
import { computed } from 'vue'
import type { TLSProbeStatus } from '@/types'

const props = defineProps<{
  status?: TLSProbeStatus
  serverId: number
  enabled: boolean
  https: boolean
  loadError?: string
}>()
const label = computed(() => {
  if (!props.enabled) return 'Disabled'
  if (!props.https) return 'HTTP only'
  if (props.serverId !== 1) return 'Check TLS on node'
  if (props.loadError) return 'TLS check unavailable'
  if (!props.status) return 'Waiting for TLS check'
  return props.status.status === 'valid'
    ? 'TLS ready'
    : props.status.status === 'pending'
      ? 'Certificate mismatch'
      : 'TLS unavailable'
})
const color = computed(() => {
  if (!props.enabled || !props.https) return 'bg-slate-100 text-slate-600'
  if (props.serverId !== 1 || !props.status || props.loadError) return 'bg-amber-100 text-amber-800'
  return props.status.status === 'valid'
    ? 'bg-emerald-100 text-emerald-700'
    : 'bg-red-100 text-red-700'
})
</script>

<template>
  <details class="mt-1 text-xs">
    <summary class="w-fit cursor-pointer rounded-full px-2 py-1 font-medium" :class="color">
      {{ label }}
    </summary>
    <div
      class="mt-2 max-w-sm space-y-2 rounded border border-slate-200 bg-white p-3 text-slate-600"
    >
      <p v-if="!enabled">This host is disabled.</p>
      <p v-else-if="!https">This host serves HTTP without requesting a certificate.</p>
      <template v-else>
        <p v-if="serverId !== 1">
          Live certificate checks are available for the local server. Inspect Caddy on this host's
          assigned node.
        </p>
        <template v-else>
          <p v-if="loadError">{{ loadError }}</p>
          <p v-else-if="!status">
            Checks run every minute. A queued or rejected configuration may not be serving yet.
          </p>
          <template v-else>
            <p v-if="status.detail">{{ status.detail }}</p>
            <p>Last checked: {{ new Date(status.checked_at).toLocaleString() }}</p>
            <ul class="space-y-1">
              <li v-for="domain in status.domains" :key="domain.domain">
                <strong>{{ domain.domain }}</strong
                >: {{ domain.status }}
                <span v-if="domain.detail"> — {{ domain.detail }}</span>
                <span v-if="domain.expires_at">
                  (expires {{ new Date(domain.expires_at).toLocaleDateString() }})</span
                >
              </li>
            </ul>
          </template>
          <p>
            These checks confirm certificate names and dates, without validating the trust chain.
            They do not report Let's Encrypt order state.
          </p>
        </template>
        <p>
          For exact Let's Encrypt issuance or renewal errors, check Caddy on the assigned server:
        </p>
        <code class="block whitespace-normal select-all"
          >sudo journalctl -u caddy -n 100 --no-pager</code
        >
        <p>
          Look for tls.obtain or tls.renew errors. In Docker, use docker logs for the controller or
          agent container.
        </p>
      </template>
    </div>
  </details>
</template>
