<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { getDashboardStats, type DashboardStats } from '@/api/dashboard'
import { getUpstreamHealth } from '@/api/health'
import type { UpstreamHealth } from '@/types'
import { useRouter } from 'vue-router'

const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(true)
const error = ref('')

// ── Upstream health ───────────────────────────────────────────────────────────
const upstreams = ref<UpstreamHealth[]>([])
const healthLoading = ref(false)
const healthLastUpdated = ref<Date | null>(null)
let healthTimer: ReturnType<typeof setInterval> | null = null

async function fetchHealth() {
  healthLoading.value = true
  try {
    upstreams.value = await getUpstreamHealth()
    healthLastUpdated.value = new Date()
  } catch {
    // non-fatal — Caddy may not be running
  } finally {
    healthLoading.value = false
  }
}

const healthyCount = computed(() => upstreams.value.filter((u) => u.status === 'healthy').length)
const unhealthyCount = computed(() => upstreams.value.filter((u) => u.status === 'unhealthy').length)
const unknownCount = computed(() => upstreams.value.filter((u) => u.status === 'unknown').length)

const lastUpdatedLabel = computed(() => {
  if (!healthLastUpdated.value) return ''
  return healthLastUpdated.value.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
})

onMounted(async () => {
  try {
    stats.value = await getDashboardStats()
  } catch {
    error.value = 'Failed to load dashboard stats.'
  } finally {
    loading.value = false
  }

  await fetchHealth()
  healthTimer = setInterval(fetchHealth, 30_000)
})

onUnmounted(() => {
  if (healthTimer !== null) clearInterval(healthTimer)
})

const totalHosts = computed(() =>
  stats.value
    ? stats.value.proxy_hosts +
      stats.value.redirection_hosts +
      stats.value.stream_hosts +
      stats.value.dead_hosts
    : 0
)

const statCards = computed(() => {
  if (!stats.value) return []
  return [
    {
      label: 'Proxy Hosts',
      value: stats.value.proxy_hosts,
      route: '/proxy-hosts',
      color: 'indigo',
    },
    {
      label: 'Redirections',
      value: stats.value.redirection_hosts,
      route: '/redirection-hosts',
      color: 'violet',
    },
    {
      label: 'Stream Hosts',
      value: stats.value.stream_hosts,
      route: '/stream-hosts',
      color: 'blue',
    },
    { label: '404 Hosts', value: stats.value.dead_hosts, route: '/dead-hosts', color: 'slate' },
    {
      label: 'Certificates',
      value: stats.value.certificates,
      route: '/certificates',
      color: 'emerald',
    },
    {
      label: 'Access Lists',
      value: stats.value.access_lists,
      route: '/access-lists',
      color: 'amber',
    },
  ]
})

const colorMap: Record<string, string> = {
  indigo: 'bg-indigo-50 text-indigo-700 border-indigo-100',
  violet: 'bg-violet-50 text-violet-700 border-violet-100',
  blue: 'bg-blue-50 text-blue-700 border-blue-100',
  slate: 'bg-slate-50 text-slate-700 border-slate-100',
  emerald: 'bg-emerald-50 text-emerald-700 border-emerald-100',
  amber: 'bg-amber-50 text-amber-700 border-amber-100',
}
</script>

<template>
  <div>
    <div class="mb-6">
      <h2 class="text-xl font-semibold text-slate-800">Dashboard</h2>
      <p class="mt-0.5 text-sm text-slate-500">
        PosternProxy — overview of your proxy infrastructure
      </p>
    </div>

    <div v-if="error" class="mb-6 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
      <div v-for="i in 6" :key="i" class="h-28 animate-pulse rounded-xl bg-slate-100" />
    </div>

    <template v-else-if="stats">
      <!-- Summary bar -->
      <div
        class="mb-6 flex items-center gap-4 rounded-xl border border-slate-200 bg-white px-6 py-4 shadow-sm"
      >
        <div class="flex-1">
          <p class="text-sm text-slate-500">Total active routes</p>
          <p class="mt-0.5 text-2xl font-bold text-slate-900">{{ stats.enabled_hosts }}</p>
        </div>
        <div class="h-10 w-px bg-slate-200" />
        <div class="flex-1">
          <p class="text-sm text-slate-500">Total host records</p>
          <p class="mt-0.5 text-2xl font-bold text-slate-900">{{ totalHosts }}</p>
        </div>
        <div class="h-10 w-px bg-slate-200" />
        <div class="flex-1">
          <p class="text-sm text-slate-500">Certificates</p>
          <p class="mt-0.5 text-2xl font-bold text-slate-900">{{ stats.certificates }}</p>
        </div>
        <div v-if="stats.expiring_soon.length" class="h-10 w-px bg-slate-200" />
        <div v-if="stats.expiring_soon.length" class="flex-1">
          <p class="text-sm text-amber-600">Expiring soon</p>
          <p class="mt-0.5 text-2xl font-bold text-amber-600">{{ stats.expiring_soon.length }}</p>
        </div>
      </div>

      <!-- Stat cards grid -->
      <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <button
          v-for="card in statCards"
          :key="card.label"
          class="rounded-xl border px-5 py-4 text-left shadow-sm transition-shadow hover:shadow-md"
          :class="colorMap[card.color]"
          @click="router.push(card.route)"
        >
          <p class="text-xs font-medium opacity-75">{{ card.label }}</p>
          <p class="mt-1.5 text-3xl font-bold">{{ card.value }}</p>
        </button>
      </div>

      <!-- ── Upstream Health widget ─────────────────────────────────────── -->
      <div class="mt-6 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
        <!-- Widget header -->
        <div class="flex items-center justify-between border-b border-slate-200 px-5 py-3">
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold text-slate-800">Upstream Health</span>
            <!-- live summary pills -->
            <template v-if="upstreams.length > 0">
              <span
                v-if="healthyCount > 0"
                class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700"
              >
                {{ healthyCount }} healthy
              </span>
              <span
                v-if="unhealthyCount > 0"
                class="rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700"
              >
                {{ unhealthyCount }} down
              </span>
              <span
                v-if="unknownCount > 0"
                class="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600"
              >
                {{ unknownCount }} no data
              </span>
            </template>
          </div>
          <div class="flex items-center gap-3">
            <span v-if="lastUpdatedLabel" class="text-xs text-slate-400">
              Updated {{ lastUpdatedLabel }}
            </span>
            <button
              class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
              :class="{ 'animate-spin': healthLoading }"
              title="Refresh"
              @click="fetchHealth"
            >
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            </button>
          </div>
        </div>

        <!-- No upstreams: health checks not enabled -->
        <div
          v-if="upstreams.length === 0 && !healthLoading"
          class="flex items-start gap-3 px-5 py-5"
        >
          <svg
            class="mt-0.5 h-5 w-5 shrink-0 text-slate-300"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <div>
            <p class="text-sm font-medium text-slate-500">No active upstream data</p>
            <p class="mt-0.5 text-xs text-slate-400">
              Enable <strong>Health Checks</strong> on one or more proxy hosts to see live upstream
              status here. Caddy polls each upstream and reports availability every 30 seconds.
            </p>
            <button
              class="mt-2 text-xs text-indigo-600 hover:underline"
              @click="router.push('/proxy-hosts')"
            >
              Go to Proxy Hosts →
            </button>
          </div>
        </div>

        <!-- Loading skeleton -->
        <div v-else-if="healthLoading && upstreams.length === 0" class="space-y-2 p-4">
          <div v-for="i in 3" :key="i" class="h-10 animate-pulse rounded-lg bg-slate-100" />
        </div>

        <!-- Upstream table -->
        <table v-else class="w-full text-left text-sm">
          <thead class="border-b border-slate-100 bg-slate-50 text-xs font-medium text-slate-500">
            <tr>
              <th class="px-5 py-2.5">Upstream</th>
              <th class="px-5 py-2.5">Status</th>
              <th class="px-5 py-2.5 text-right">Requests</th>
              <th class="px-5 py-2.5 text-right">Failures</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr
              v-for="u in upstreams"
              :key="u.address"
              class="transition-colors"
              :class="u.status === 'unhealthy' ? 'bg-red-50/60 hover:bg-red-50' : 'hover:bg-slate-50'"
            >
              <!-- Address -->
              <td class="px-5 py-3 font-mono text-xs text-slate-700">{{ u.address }}</td>

              <!-- Status pill -->
              <td class="px-5 py-3">
                <span
                  class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
                  :class="{
                    'bg-emerald-100 text-emerald-700': u.status === 'healthy',
                    'bg-red-100 text-red-700': u.status === 'unhealthy',
                    'bg-slate-100 text-slate-600': u.status === 'unknown',
                  }"
                >
                  <span
                    class="h-1.5 w-1.5 rounded-full"
                    :class="{
                      'bg-emerald-500': u.status === 'healthy',
                      'bg-red-500 animate-pulse': u.status === 'unhealthy',
                      'bg-slate-400': u.status === 'unknown',
                    }"
                  />
                  {{
                    u.status === 'healthy'
                      ? 'Healthy'
                      : u.status === 'unhealthy'
                        ? 'Unhealthy'
                        : 'No data'
                  }}
                </span>
              </td>

              <!-- Request count -->
              <td class="px-5 py-3 text-right tabular-nums text-slate-600">
                {{ u.num_requests.toLocaleString() }}
              </td>

              <!-- Fail count -->
              <td
                class="px-5 py-3 text-right tabular-nums"
                :class="u.fails > 0 ? 'font-semibold text-red-600' : 'text-slate-400'"
              >
                {{ u.fails > 0 ? u.fails.toLocaleString() : '—' }}
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Unhealthy footer -->
        <div
          v-if="upstreams.length > 0 && unhealthyCount > 0"
          class="border-t border-red-100 bg-red-50 px-5 py-2 text-xs font-medium text-red-700"
        >
          ⚠ {{ unhealthyCount }} upstream{{ unhealthyCount !== 1 ? 's are' : ' is' }} unhealthy —
          Caddy is routing around them
        </div>
        <!-- All healthy footer (only when every upstream has a positive signal) -->
        <div
          v-else-if="upstreams.length > 0 && unknownCount === 0"
          class="border-t border-slate-100 px-5 py-2 text-xs text-emerald-600"
        >
          ✓ All {{ upstreams.length }} upstream{{ upstreams.length !== 1 ? 's' : '' }} healthy —
          polled every 30 s
        </div>
        <!-- No-data footer: upstreams exist but none has served traffic yet -->
        <div
          v-else-if="upstreams.length > 0"
          class="border-t border-slate-100 px-5 py-2 text-xs text-slate-500"
        >
          {{ unknownCount }} upstream{{ unknownCount !== 1 ? 's have' : ' has' }} no observed traffic
          yet — status is unknown until requests flow or active health checks are enabled.
        </div>
      </div>

      <!-- Expiring certificates alert -->
      <div
        v-if="stats.expiring_soon.length"
        class="mt-6 overflow-hidden rounded-xl border border-amber-200 bg-amber-50 shadow-sm"
      >
        <div class="border-b border-amber-200 px-5 py-3">
          <p class="text-sm font-semibold text-amber-800">
            ⚠ {{ stats.expiring_soon.length }} certificate{{
              stats.expiring_soon.length !== 1 ? 's' : ''
            }}
            expiring within 30 days
          </p>
        </div>
        <table class="w-full text-left text-sm">
          <tbody class="divide-y divide-amber-100">
            <tr v-for="cert in stats.expiring_soon" :key="cert.id" class="hover:bg-amber-100/40">
              <td class="px-5 py-2.5 font-medium text-amber-900">{{ cert.name }}</td>
              <td class="px-5 py-2.5 text-amber-700">{{ cert.expires_at.slice(0, 10) }}</td>
              <td class="px-5 py-2.5">
                <span
                  class="rounded-full px-2 py-0.5 text-xs font-semibold"
                  :class="
                    cert.days_left <= 7 ? 'bg-red-200 text-red-800' : 'bg-amber-200 text-amber-800'
                  "
                >
                  {{ cert.days_left }}d left
                </span>
              </td>
              <td class="px-5 py-2.5 text-right">
                <button
                  class="text-xs text-amber-700 underline hover:text-amber-900"
                  @click="router.push('/certificates')"
                >
                  Manage →
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Empty state -->
      <div
        v-if="totalHosts === 0"
        class="mt-6 rounded-xl border border-dashed border-slate-300 bg-white p-12 text-center"
      >
        <p class="text-sm font-medium text-slate-500">No hosts configured yet.</p>
        <p class="mt-1 text-xs text-slate-400">Get started by adding a proxy host.</p>
        <button
          class="mt-4 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
          @click="router.push({ name: 'proxy-hosts', query: { create: '1' } })"
        >
          Add Proxy Host
        </button>
      </div>
    </template>
  </div>
</template>
