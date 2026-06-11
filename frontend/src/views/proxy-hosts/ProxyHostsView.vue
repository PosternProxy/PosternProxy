<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type { ProxyHost, AccessList, Certificate, UpstreamHealth } from '@/types'
import {
  listProxyHosts,
  deleteProxyHost,
  enableProxyHost,
  disableProxyHost,
} from '@/api/proxyHosts'
import { listAccessLists } from '@/api/accessLists'
import { listCertificates } from '@/api/certificates'
import { getUpstreamHealth } from '@/api/health'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'
import ConfigHistoryModal from '@/components/ConfigHistoryModal.vue'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'
import ProxyHostFormModal from './ProxyHostFormModal.vue'

const serverStore = useServerStore()
const ctx = useServerContext()

// ── Hosts list + delete-confirm via composable ───────────────────────────────

const {
  items: hosts,
  loading,
  loadError: error,
  load: fetchHosts,
  showDeleteConfirm,
  deleting,
  deleteError,
  confirmDelete,
  doDelete,
  upsert,
  appendItem,
} = useResourceList<ProxyHost>({
  list: () => listProxyHosts(ctx.selectedServerId),
  delete: deleteProxyHost,
  itemNoun: 'proxy host',
})

// ── Reference data for the form modal ────────────────────────────────────────

const accessLists = ref<AccessList[]>([])
const certificates = ref<Certificate[]>([])

async function fetchAccessLists() {
  try {
    accessLists.value = await listAccessLists()
  } catch {
    /* non-fatal */
  }
}
async function fetchCertificates() {
  try {
    certificates.value = await listCertificates()
  } catch {
    /* non-fatal */
  }
}

// ── Upstream health (polled every 30 s) ──────────────────────────────────────
// Lives in the parent because the table column shows live health per row.

const upstreamHealth = ref<UpstreamHealth[]>([])
let healthTimer: ReturnType<typeof setInterval> | null = null

const healthMap = computed(() => {
  const m = new Map<string, UpstreamHealth>()
  for (const u of upstreamHealth.value) {
    m.set(u.address, u)
  }
  return m
})

async function fetchHealth() {
  try {
    upstreamHealth.value = await getUpstreamHealth()
  } catch {
    /* non-fatal — Caddy might not be running */
  }
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

onMounted(async () => {
  await serverStore.ensureLoaded()
  await Promise.all([fetchHosts(), fetchAccessLists(), fetchCertificates(), fetchHealth()])
  healthTimer = setInterval(fetchHealth, 30_000)
})

onUnmounted(() => {
  if (healthTimer !== null) clearInterval(healthTimer)
})

watch(() => ctx.selectedServerId, fetchHosts)

// ── Form modal wiring ────────────────────────────────────────────────────────
// editingHost: null = create mode; ProxyHost = edit mode.

const showFormModal = ref(false)
const editingHost = ref<ProxyHost | null>(null)

function openCreate() {
  editingHost.value = null
  showFormModal.value = true
}

function openEdit(host: ProxyHost) {
  editingHost.value = host
  showFormModal.value = true
}

function onSaved(host: ProxyHost) {
  // upsert handles both create (appends) and edit (replaces).
  if (editingHost.value === null) {
    appendItem(host)
  } else {
    upsert(host)
  }
}

// ── Enable / Disable ─────────────────────────────────────────────────────────

async function toggleEnabled(host: ProxyHost) {
  try {
    const updated = host.enabled ? await disableProxyHost(host.id) : await enableProxyHost(host.id)
    upsert(updated)
  } catch {
    // silently ignore; could show toast here
  }
}

// ── History modal ────────────────────────────────────────────────────────────

const showHistoryModal = ref(false)
const historyHostId = ref<number>(0)

function openHistory(host: ProxyHost) {
  historyHostId.value = host.id
  showHistoryModal.value = true
}
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Proxy Hosts</h2>
        <p class="mt-0.5 text-sm text-slate-500">Route incoming domains to upstream services</p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="openCreate"
      >
        + Add Proxy Host
      </button>
    </div>

    <!-- Error -->
    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!loading && hosts.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <svg
        class="mx-auto mb-3 h-10 w-10 text-slate-300"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9"
        />
      </svg>
      <p class="text-sm font-medium text-slate-500">No proxy hosts configured yet.</p>
      <p class="mt-1 text-xs text-slate-400">Click "Add Proxy Host" to get started.</p>
    </div>

    <!-- Table -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Domain Names</th>
            <th class="px-4 py-3 font-medium text-slate-600">Upstream</th>
            <th class="px-4 py-3 font-medium text-slate-600">Health</th>
            <th class="px-4 py-3 font-medium text-slate-600">SSL</th>
            <th class="px-4 py-3 font-medium text-slate-600">Status</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr
            v-for="host in hosts"
            :key="host.id"
            class="transition-colors hover:bg-slate-50"
            :class="{ 'opacity-50': !host.enabled }"
          >
            <!-- Domains -->
            <td class="px-4 py-3">
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="d in host.domain_names"
                  :key="d"
                  class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700"
                  >{{ d }}</span
                >
              </div>
              <!-- Security badges -->
              <div class="mt-1 flex flex-wrap gap-1">
                <span
                  v-if="host.tls_passthrough"
                  class="rounded-full bg-purple-100 px-1.5 py-0.5 text-xs font-medium text-purple-700"
                  >TLS Pass</span
                >
                <span
                  v-if="host.rate_limit_enabled"
                  class="rounded-full bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-700"
                  >Rate Limited</span
                >
                <span
                  v-if="host.extra_upstreams?.length > 0"
                  class="rounded-full bg-blue-100 px-1.5 py-0.5 text-xs font-medium text-blue-700"
                  >{{ host.extra_upstreams.length + 1 }} upstreams</span
                >
              </div>
            </td>

            <!-- Upstream -->
            <td class="px-4 py-3 font-mono text-xs text-slate-600">
              {{ host.forward_scheme }}://{{ host.forward_host }}:{{ host.forward_port }}
            </td>

            <!-- Health indicator -->
            <td class="px-4 py-3">
              <template v-if="!host.hc_enabled">
                <span class="text-xs text-slate-400">—</span>
              </template>
              <template v-else>
                <!-- Look up live status from Caddy -->
                <template v-if="healthMap.has(`${host.forward_host}:${host.forward_port}`)">
                  <span
                    class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="
                      healthMap.get(`${host.forward_host}:${host.forward_port}`)!.healthy
                        ? 'bg-emerald-100 text-emerald-700'
                        : 'bg-red-100 text-red-700'
                    "
                  >
                    <span
                      class="h-1.5 w-1.5 rounded-full"
                      :class="
                        healthMap.get(`${host.forward_host}:${host.forward_port}`)!.healthy
                          ? 'bg-emerald-500'
                          : 'bg-red-500'
                      "
                    />
                    {{
                      healthMap.get(`${host.forward_host}:${host.forward_port}`)!.healthy
                        ? 'Healthy'
                        : `${healthMap.get(`${host.forward_host}:${host.forward_port}`)!.fails} fail${healthMap.get(`${host.forward_host}:${host.forward_port}`)!.fails === 1 ? '' : 's'}`
                    }}
                  </span>
                </template>
                <template v-else>
                  <span
                    class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700"
                  >
                    <span class="h-1.5 w-1.5 rounded-full bg-amber-400" />
                    Pending
                  </span>
                </template>
              </template>
            </td>

            <!-- SSL badge -->
            <td class="px-4 py-3">
              <span
                v-if="host.tls_passthrough"
                class="inline-flex items-center gap-1 rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700"
              >
                <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                Passthrough
              </span>
              <span
                v-else-if="host.http_only"
                class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700"
              >
                HTTP
              </span>
              <span
                v-else-if="host.certificate_id"
                class="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700"
              >
                <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fill-rule="evenodd"
                    d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                    clip-rule="evenodd"
                  />
                </svg>
                SSL
              </span>
              <span
                v-else
                class="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700"
                >Auto</span
              >
            </td>

            <!-- Status toggle -->
            <td class="px-4 py-3">
              <button
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors"
                :class="
                  host.enabled
                    ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200'
                    : 'bg-slate-100 text-slate-500 hover:bg-slate-200'
                "
                @click="toggleEnabled(host)"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="host.enabled ? 'bg-emerald-500' : 'bg-slate-400'"
                />
                {{ host.enabled ? 'Online' : 'Offline' }}
              </button>
            </td>

            <!-- Actions -->
            <td class="px-4 py-3 text-right">
              <div class="flex items-center justify-end gap-2">
                <!-- History button -->
                <button
                  class="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                  title="Configuration History"
                  @click="openHistory(host)"
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </button>
                <button
                  class="rounded px-2 py-1 text-xs text-indigo-600 hover:bg-indigo-50"
                  @click="openEdit(host)"
                >
                  Edit
                </button>
                <button
                  class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
                  @click="confirmDelete(host.id)"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Create / Edit form modal ──────────────────────────────────────── -->
    <ProxyHostFormModal
      v-model="showFormModal"
      :host="editingHost"
      :default-server-id="ctx.selectedServerId"
      :access-lists="accessLists"
      :certificates="certificates"
      @saved="onSaved"
    />

    <!-- ── Delete Confirmation ─────────────────────────────────────────────── -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Delete Proxy Host"
      message="This will permanently remove the proxy host and stop routing traffic for its domains. This cannot be undone."
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />

    <!-- ── Config History Modal ───────────────────────────────────────────── -->
    <ConfigHistoryModal
      :show="showHistoryModal"
      resource-type="proxy_host"
      :resource-id="historyHostId"
      @close="showHistoryModal = false"
      @rolled-back="fetchHosts"
    />
  </div>
</template>
