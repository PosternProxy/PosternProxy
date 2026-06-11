<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, nextTick } from 'vue'
import type { Server } from '@/types'
import {
  listServers,
  createServer,
  updateServer,
  deleteServer,
  resyncServer,
  type CreateServerInput,
} from '@/api/servers'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'

// ── State ─────────────────────────────────────────────────────────────────────

// useResourceList covers the load + delete-confirm slice; ServersView's wizard
// and edit modal are bespoke (multi-step provisioning, SSE log streaming) and
// stay local.
const {
  items: servers,
  loading,
  loadError: error,
  load: fetchServers,
  showDeleteConfirm,
  deleting,
  deleteError,
  confirmDelete,
  doDelete,
  upsert,
} = useResourceList<Server>({
  list: listServers,
  delete: deleteServer,
  itemNoun: 'server',
})

// Add wizard (3-step)
const showWizard = ref(false)
const wizardStep = ref<1 | 2 | 3>(1) // 1=credentials, 2=provisioning, 3=done
const wizardSaving = ref(false)
const wizardError = ref('')
const provisionStatus = ref<'running' | 'online' | 'error' | 'timeout'>('running')
const provisionLines = ref<string[]>([])
const provisionLogEl = ref<HTMLElement | null>(null)
let eventSource: EventSource | null = null

// Edit modal (separate from useResourceList's create-modal slice)
const showEditModal = ref(false)
const editingId = ref<number | null>(null)
const editSaving = ref(false)
const editError = ref('')

// Re-sync state
const resyncingId = ref<number | null>(null)
const resyncSuccessId = ref<number | null>(null)
const resyncError = ref('')

// ── Form ──────────────────────────────────────────────────────────────────────

const defaultForm = (): CreateServerInput => ({
  name: '',
  hostname: '',
  ssh_port: 22,
  ssh_user: 'root',
  ssh_auth_method: 'password',
  ssh_credentials: '',
  deployment_type: 'systemd',
})

const form = reactive(defaultForm())

// ── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(fetchServers)
onUnmounted(() => eventSource?.close())

// ── Add wizard ────────────────────────────────────────────────────────────────

function openWizard() {
  Object.assign(form, defaultForm())
  wizardStep.value = 1
  wizardError.value = ''
  provisionLines.value = []
  provisionStatus.value = 'running'
  showWizard.value = true
}

function closeWizard() {
  eventSource?.close()
  eventSource = null
  showWizard.value = false
  if (wizardStep.value === 3 && provisionStatus.value === 'online') {
    fetchServers()
  }
}

async function submitCredentials() {
  if (!form.name || !form.hostname || !form.ssh_credentials) {
    wizardError.value = 'Name, hostname, and credentials are required.'
    return
  }
  wizardSaving.value = true
  wizardError.value = ''
  try {
    const srv = await createServer({ ...form })
    // srv.id is captured implicitly by closures below
    servers.value.push(srv)
    wizardStep.value = 2
    startProvisionStream(srv.id)
  } catch (e: unknown) {
    wizardError.value = e instanceof Error ? e.message : 'Failed to create server'
  } finally {
    wizardSaving.value = false
  }
}

function startProvisionStream(id: number) {
  provisionLines.value = []
  provisionStatus.value = 'running'
  streamProvisionLog(id)
}

async function streamProvisionLog(id: number) {
  try {
    const { useAuthStore } = await import('@/stores/auth')
    const auth = useAuthStore()
    const resp = await fetch(`/api/servers/${id}/provision-log`, {
      headers: {
        Authorization: `Bearer ${auth.token}`,
        Accept: 'text/event-stream',
      },
    })
    if (!resp.ok || !resp.body) {
      provisionStatus.value = 'error'
      provisionLines.value.push(`[error] HTTP ${resp.status}`)
      wizardStep.value = 3
      return
    }

    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const parts = buf.split('\n')
      buf = parts.pop() ?? ''

      for (const part of parts) {
        const trimmed = part.trim()
        if (trimmed.startsWith('data:')) {
          provisionLines.value.push(trimmed.slice(5).trim())
          await nextTick()
          if (provisionLogEl.value) {
            provisionLogEl.value.scrollTop = provisionLogEl.value.scrollHeight
          }
        } else if (trimmed.startsWith('event: done')) {
          // Next data line carries the status
        } else if (trimmed.startsWith('data: online') || trimmed === 'data: online') {
          provisionStatus.value = 'online'
          wizardStep.value = 3
          syncServerStatus(id)
        } else if (trimmed.startsWith('data: error') || trimmed === 'data: error') {
          provisionStatus.value = 'error'
          wizardStep.value = 3
          syncServerStatus(id)
        } else if (trimmed.startsWith('data: timeout')) {
          provisionStatus.value = 'timeout'
          wizardStep.value = 3
        }
      }
    }
  } catch (e) {
    provisionStatus.value = 'error'
    provisionLines.value.push(`[error] ${e}`)
    wizardStep.value = 3
  }
}

async function syncServerStatus(id: number) {
  try {
    const updated = await import('@/api/servers').then((m) => m.getServer(id))
    const idx = servers.value.findIndex((s) => s.id === id)
    if (idx >= 0) servers.value[idx] = updated
  } catch {
    /* non-fatal */
  }
}

// ── Re-sync ───────────────────────────────────────────────────────────────────

async function doResync(srv: Server) {
  resyncingId.value = srv.id
  resyncSuccessId.value = null
  resyncError.value = ''
  try {
    await resyncServer(srv.id)
    resyncSuccessId.value = srv.id
    setTimeout(() => {
      if (resyncSuccessId.value === srv.id) resyncSuccessId.value = null
    }, 2000)
  } catch (e: unknown) {
    resyncError.value = e instanceof Error ? e.message : 'Resync failed'
    setTimeout(() => {
      resyncError.value = ''
    }, 4000)
  } finally {
    resyncingId.value = null
  }
}

// ── Redeploy ──────────────────────────────────────────────────────────────────

function openRedeploy(srv: Server) {
  // srv.id is captured implicitly by closures below
  provisionLines.value = []
  provisionStatus.value = 'running'
  wizardStep.value = 2
  showWizard.value = true
  startProvisionStream(srv.id)
}

// ── Edit modal ────────────────────────────────────────────────────────────────

function openEdit(srv: Server) {
  editingId.value = srv.id
  editError.value = ''
  Object.assign(form, {
    name: srv.name,
    hostname: srv.hostname,
    ssh_port: srv.ssh_port,
    ssh_user: srv.ssh_user,
    ssh_auth_method: srv.ssh_auth_method,
    ssh_credentials: '',
  })
  showEditModal.value = true
}

function closeEdit() {
  showEditModal.value = false
}

async function saveEdit() {
  if (!form.name) {
    editError.value = 'Name is required.'
    return
  }
  editSaving.value = true
  editError.value = ''
  try {
    const updated = await updateServer(editingId.value!, {
      name: form.name,
      ssh_port: form.ssh_port,
      ssh_user: form.ssh_user,
      ssh_auth_method: form.ssh_auth_method,
      ssh_credentials: form.ssh_credentials,
    })
    upsert(updated)
    closeEdit()
  } catch (e: unknown) {
    editError.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    editSaving.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────────────────
// Delete state + handlers come from useResourceList above. The L-2 pattern
// (keep modal open + show error + leave row in the list on failure) is
// encapsulated by the composable's doDelete().

// ── Helpers ───────────────────────────────────────────────────────────────────

function statusBadge(srv: Server) {
  if (srv.is_local) return { cls: 'bg-indigo-100 text-indigo-700', label: 'Local' }
  if (srv.online) return { cls: 'bg-emerald-100 text-emerald-700', label: 'Online' }
  const map: Record<string, { cls: string; label: string }> = {
    provisioning: { cls: 'bg-amber-100 text-amber-700', label: 'Provisioning' },
    error: { cls: 'bg-red-100 text-red-700', label: 'Error' },
    offline: { cls: 'bg-slate-100 text-slate-600', label: 'Offline' },
    pending: { cls: 'bg-slate-100 text-slate-500', label: 'Pending' },
  }
  return map[srv.status] ?? { cls: 'bg-slate-100 text-slate-500', label: srv.status }
}

function formatUptime(seconds: number): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Servers</h2>
        <p class="mt-0.5 text-sm text-slate-500">Manage proxy servers — local and remote agents</p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="openWizard"
      >
        + Add Server
      </button>
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>
    <div v-if="resyncError" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
      Resync failed: {{ resyncError }}
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 2" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>

    <!-- Table -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Name</th>
            <th class="px-4 py-3 font-medium text-slate-600">Status</th>
            <th class="px-4 py-3 font-medium text-slate-600">Caddy</th>
            <th class="px-4 py-3 font-medium text-slate-600">Uptime</th>
            <th class="px-4 py-3 font-medium text-slate-600">Routes</th>
            <th class="px-4 py-3 font-medium text-slate-600">Last Seen</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="srv in servers" :key="srv.id" class="transition-colors hover:bg-slate-50">
            <td class="px-4 py-3 font-medium text-slate-800">
              {{ srv.name }}
              <span v-if="srv.is_local" class="ml-1.5 text-xs text-slate-400">(controller)</span>
              <div class="mt-0.5 font-mono text-xs font-normal text-slate-400">
                {{ srv.hostname }}
              </div>
            </td>
            <td class="px-4 py-3">
              <span
                class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
                :class="statusBadge(srv).cls"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="srv.online || srv.is_local ? 'bg-emerald-500' : 'bg-slate-400'"
                />
                {{ statusBadge(srv).label }}
              </span>
            </td>
            <td class="px-4 py-3 font-mono text-xs text-slate-500">
              {{ srv.caddy_version || '—' }}
            </td>
            <td class="px-4 py-3 text-xs text-slate-500">
              {{ formatUptime(srv.uptime_seconds) }}
            </td>
            <td class="px-4 py-3 text-xs text-slate-500">
              {{ srv.active_routes > 0 ? srv.active_routes : srv.online ? '0' : '—' }}
            </td>
            <td class="px-4 py-3 text-xs text-slate-500">
              {{ srv.last_seen_at ? srv.last_seen_at.slice(0, 16).replace('T', ' ') : '—' }}
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex items-center justify-end gap-1.5">
                <!-- Re-sync -->
                <button
                  v-if="!srv.is_local"
                  class="rounded px-2 py-1 text-xs font-medium transition-colors"
                  :class="
                    resyncSuccessId === srv.id
                      ? 'bg-emerald-100 text-emerald-700'
                      : 'text-indigo-600 hover:bg-indigo-50 disabled:cursor-not-allowed disabled:opacity-40'
                  "
                  :disabled="!srv.online || resyncingId === srv.id"
                  :title="
                    srv.online ? 'Push current config to agent' : 'Agent must be online to resync'
                  "
                  @click="doResync(srv)"
                >
                  <span v-if="resyncingId === srv.id">Syncing…</span>
                  <span v-else-if="resyncSuccessId === srv.id">✓ Synced</span>
                  <span v-else>Re-sync</span>
                </button>

                <!-- Redeploy -->
                <button
                  v-if="!srv.is_local"
                  class="rounded px-2 py-1 text-xs font-medium text-amber-600 hover:bg-amber-50"
                  title="Re-run SSH provisioner"
                  @click="openRedeploy(srv)"
                >
                  Redeploy
                </button>

                <button
                  v-if="!srv.is_local"
                  class="rounded px-2 py-1 text-xs text-slate-600 hover:bg-slate-100"
                  @click="openEdit(srv)"
                >
                  Edit
                </button>
                <button
                  v-if="!srv.is_local"
                  class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
                  @click="confirmDelete(srv.id)"
                >
                  Delete
                </button>
                <span v-if="srv.is_local" class="text-xs text-slate-400">—</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Add / Redeploy Wizard ─────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showWizard"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="wizardStep === 1 || wizardStep === 3 ? closeWizard() : undefined"
      >
        <div class="flex w-full max-w-2xl flex-col rounded-2xl bg-white shadow-2xl">
          <!-- Wizard header + step indicator -->
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <div>
              <h3 class="text-base font-semibold text-slate-800">
                {{ wizardStep === 1 ? 'Add Remote Server' : 'Provisioning' }}
              </h3>
              <div class="mt-1 flex items-center gap-2">
                <span
                  v-for="(label, i) in ['SSH Credentials', 'Provisioning', 'Complete']"
                  :key="i"
                  class="flex items-center gap-1.5 text-xs"
                >
                  <span
                    class="flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-bold"
                    :class="
                      wizardStep > i + 1
                        ? 'bg-emerald-500 text-white'
                        : wizardStep === i + 1
                          ? 'bg-indigo-600 text-white'
                          : 'bg-slate-200 text-slate-500'
                    "
                    >{{ i + 1 }}</span
                  >
                  <span
                    :class="wizardStep === i + 1 ? 'text-indigo-700 font-medium' : 'text-slate-400'"
                  >
                    {{ label }}
                  </span>
                  <span v-if="i < 2" class="text-slate-300">›</span>
                </span>
              </div>
            </div>
            <button
              v-if="wizardStep === 1 || wizardStep === 3"
              class="rounded-lg p-1 text-slate-400 hover:bg-slate-100"
              @click="closeWizard"
            >
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <!-- Step 1: SSH credentials -->
          <div
            v-if="wizardStep === 1"
            class="space-y-4 overflow-y-auto px-6 py-5"
            style="max-height: 65vh"
          >
            <div v-if="wizardError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ wizardError }}
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="col-span-2">
                <label class="mb-1 block text-sm font-medium text-slate-700">Server Name</label>
                <input
                  v-model="form.name"
                  type="text"
                  placeholder="prod-proxy-01"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div class="col-span-2">
                <label class="mb-1 block text-sm font-medium text-slate-700">Hostname / IP</label>
                <input
                  v-model="form.hostname"
                  type="text"
                  placeholder="203.0.113.10"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">SSH User</label>
                <input
                  v-model="form.ssh_user"
                  type="text"
                  placeholder="root"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">SSH Port</label>
                <input
                  v-model.number="form.ssh_port"
                  type="number"
                  min="1"
                  max="65535"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>

            <!-- Auth method toggle -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">Authentication</label>
              <div class="flex gap-2">
                <button
                  v-for="m in [
                    { value: 'password', label: 'Password' },
                    { value: 'key', label: 'Private Key' },
                  ]"
                  :key="m.value"
                  class="rounded-lg border px-4 py-1.5 text-sm font-medium transition-colors"
                  :class="
                    form.ssh_auth_method === m.value
                      ? 'border-indigo-600 bg-indigo-50 text-indigo-700'
                      : 'border-slate-200 text-slate-600 hover:border-slate-300'
                  "
                  @click="form.ssh_auth_method = m.value as 'password' | 'key'"
                >
                  {{ m.label }}
                </button>
              </div>
            </div>

            <div v-if="form.ssh_auth_method === 'password'">
              <label class="mb-1 block text-sm font-medium text-slate-700">Password</label>
              <input
                v-model="form.ssh_credentials"
                type="password"
                autocomplete="new-password"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
            <div v-else>
              <label class="mb-1 block text-sm font-medium text-slate-700">Private Key (PEM)</label>
              <textarea
                v-model="form.ssh_credentials"
                rows="6"
                spellcheck="false"
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;..."
                class="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-slate-400">
                Paste your private key. It will be encrypted before being stored.
              </p>
            </div>

            <!-- Deployment type -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">Deployment</label>
              <div class="flex gap-2">
                <button
                  v-for="m in [
                    { value: 'systemd', label: 'Bare Metal (systemd)' },
                    { value: 'docker', label: 'Docker Container' },
                  ]"
                  :key="m.value"
                  class="rounded-lg border px-4 py-1.5 text-sm font-medium transition-colors"
                  :class="
                    form.deployment_type === m.value
                      ? 'border-indigo-600 bg-indigo-50 text-indigo-700'
                      : 'border-slate-200 text-slate-600 hover:border-slate-300'
                  "
                  @click="form.deployment_type = m.value as 'systemd' | 'docker'"
                >
                  {{ m.label }}
                </button>
              </div>
            </div>

            <div
              class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-700"
            >
              <template v-if="form.deployment_type === 'systemd'">
                <span class="font-medium">What happens next:</span> PosternProxy will SSH into the
                server, install Caddy (with L4 module), deploy the agent binary, and configure
                systemd + OS hardening. This takes 5–10 minutes depending on internet speed.
              </template>
              <template v-else>
                <span class="font-medium">What happens next:</span> PosternProxy will SSH into the
                server, install Docker if not already present, and start the agent as a container.
                No build tools required — typically completes in under 2 minutes.
              </template>
            </div>
          </div>

          <!-- Step 2: Provisioning log stream -->
          <div v-else-if="wizardStep === 2" class="px-6 py-5">
            <p class="mb-3 text-sm text-slate-600">
              Provisioning in progress — do not close this window.
            </p>
            <div
              ref="provisionLogEl"
              class="h-80 overflow-y-auto rounded-lg bg-slate-900 p-4 font-mono text-xs leading-relaxed text-emerald-400"
            >
              <div v-for="(line, i) in provisionLines" :key="i" class="whitespace-pre-wrap">
                {{ line }}
              </div>
              <span class="animate-pulse">█</span>
            </div>
          </div>

          <!-- Step 3: Result -->
          <div v-else class="px-6 py-5 text-center">
            <div v-if="provisionStatus === 'online'">
              <div
                class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100"
              >
                <svg
                  class="h-8 w-8 text-emerald-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </div>
              <p class="text-base font-semibold text-slate-800">Server provisioned successfully!</p>
              <p class="mt-1 text-sm text-slate-500">
                The agent is connected and the server is online.
              </p>
            </div>
            <div v-else>
              <div
                class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-red-100"
              >
                <svg
                  class="h-8 w-8 text-red-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </div>
              <p class="text-base font-semibold text-slate-800">Provisioning failed</p>
              <p class="mt-1 text-sm text-slate-500">
                {{
                  provisionStatus === 'timeout'
                    ? 'Provisioning timed out.'
                    : 'See the log above for details.'
                }}
              </p>
              <div
                class="mt-4 max-h-48 overflow-y-auto rounded-lg bg-slate-900 p-3 text-left font-mono text-xs text-red-400"
              >
                <div v-for="(line, i) in provisionLines" :key="i">{{ line }}</div>
              </div>
            </div>
          </div>

          <!-- Wizard footer -->
          <div class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4">
            <template v-if="wizardStep === 1">
              <button
                class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                @click="closeWizard"
              >
                Cancel
              </button>
              <button
                class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                :disabled="wizardSaving"
                @click="submitCredentials"
              >
                {{ wizardSaving ? 'Connecting…' : 'Start Provisioning →' }}
              </button>
            </template>
            <template v-else-if="wizardStep === 2">
              <span class="text-sm text-slate-500">Installing… please wait</span>
            </template>
            <template v-else>
              <button
                class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
                @click="closeWizard"
              >
                Close
              </button>
            </template>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ── Edit Modal ──────────────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showEditModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeEdit"
      >
        <div class="flex w-full max-w-lg flex-col rounded-2xl bg-white shadow-2xl">
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">Edit Server</h3>
            <button class="rounded-lg p-1 text-slate-400 hover:bg-slate-100" @click="closeEdit">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <div class="space-y-4 px-6 py-5">
            <div v-if="editError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ editError }}
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Name</label>
              <input
                v-model="form.name"
                type="text"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">SSH User</label>
                <input
                  v-model="form.ssh_user"
                  type="text"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">SSH Port</label>
                <input
                  v-model.number="form.ssh_port"
                  type="number"
                  min="1"
                  max="65535"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">
                Update Credentials
                <span class="text-xs font-normal text-slate-400"
                  >(leave blank to keep existing)</span
                >
              </label>
              <textarea
                v-model="form.ssh_credentials"
                rows="3"
                spellcheck="false"
                placeholder="New password or private key (optional)"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
          </div>
          <div class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4">
            <button
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
              :disabled="editSaving"
              @click="closeEdit"
            >
              Cancel
            </button>
            <button
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
              :disabled="editSaving"
              @click="saveEdit"
            >
              {{ editSaving ? 'Saving…' : 'Save Changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ── Delete Confirm ─────────────────────────────────────────────────── -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Remove Server"
      message="This will remove the server record and disconnect any active agent. Host configurations assigned to this server will remain but become unreachable."
      confirm-label="Remove Server"
      busy-label="Removing…"
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
