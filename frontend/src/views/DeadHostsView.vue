<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { listCertificates } from '@/api/certificates'
import { listAccessLists } from '@/api/accessLists'
import type { Certificate, AccessList, DeadHost } from '@/types'
import {
  listDeadHosts,
  createDeadHost,
  updateDeadHost,
  deleteDeadHost,
  enableDeadHost,
  disableDeadHost,
  type CreateDeadHostInput,
} from '@/api/deadHosts'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'
import TLSStatusBadge from '@/components/TLSStatusBadge.vue'
import { useTLSStatus } from '@/composables/useTLSStatus'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'

const certificates = ref<Certificate[]>([])
const accessLists = ref<AccessList[]>([])
const serverStore = useServerStore()
const ctx = useServerContext()
const { tlsFor, tlsError } = useTLSStatus('dead_host')

const {
  items: hosts,
  loading,
  loadError: error,
  load: fetchHosts,
  showModal,
  editingId,
  modalMode,
  saving,
  saveError,
  openCreate: baseOpenCreate,
  openEdit: baseOpenEdit,
  closeModal,
  showDeleteConfirm,
  deleting,
  deleteError,
  confirmDelete,
  doDelete,
  upsert,
  appendItem,
} = useResourceList<DeadHost>({
  list: () => listDeadHosts(ctx.selectedServerId),
  delete: deleteDeadHost,
  itemNoun: 'host',
})

const statusOptions: { code: 200 | 404 | 410 | 503; label: string; description: string }[] = [
  {
    code: 200,
    label: '200 Custom Page',
    description: 'Serve your own HTML. Good for coming soon, maintenance, or placeholder pages.',
  },
  {
    code: 404,
    label: '404 Not Found',
    description: 'Domain exists but has no content. Use when it may return in the future.',
  },
  {
    code: 410,
    label: '410 Gone',
    description: 'Content permanently removed. Tells search engines to deindex immediately.',
  },
  {
    code: 503,
    label: '503 Unavailable',
    description: 'Temporarily down. Search engines keep the page indexed; browsers will retry.',
  },
]

const starterHtml = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Coming Soon</title>
  <style>
    body { font-family: sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #f8fafc; }
    .card { text-align: center; padding: 2rem; }
    h1 { color: #1e293b; margin-bottom: 0.5rem; }
    p { color: #64748b; margin: 0; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Coming Soon</h1>
    <p>This site is under construction.</p>
  </div>
</body>
</html>`

const defaultForm = () => ({
  http_only: false,
  certificate_id: null as number | null,
  access_list_id: null as number | null,
  domainInput: '',
  domain_names: [] as string[],
  server_id: 1,
  status_code: 404 as 200 | 404 | 410 | 503,
  custom_body: '',
  useCustomBody: false,
})
const form = reactive(defaultForm())

onMounted(async () => {
  await serverStore.ensureLoaded()
  await fetchHosts()
  certificates.value = await listCertificates()
  accessLists.value = await listAccessLists()
})

watch(() => ctx.selectedServerId, fetchHosts)

function addDomain() {
  const v = form.domainInput.trim()
  if (v && !form.domain_names.includes(v)) form.domain_names.push(v)
  form.domainInput = ''
}
function removeDomain(i: number) {
  form.domain_names.splice(i, 1)
}
function domainKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    addDomain()
  } else if (e.key === 'Backspace' && form.domainInput === '' && form.domain_names.length)
    form.domain_names.pop()
}

function openCreate() {
  Object.assign(form, { ...defaultForm(), server_id: ctx.selectedServerId })
  baseOpenCreate()
}
function openEdit(host: DeadHost) {
  Object.assign(form, {
    domainInput: '',
    domain_names: [...host.domain_names],
    http_only: host.http_only,
    certificate_id: host.certificate_id,
    access_list_id: host.access_list_id,
    server_id: host.server_id ?? 1,
    status_code: host.status_code ?? 404,
    custom_body: host.custom_body ?? '',
    useCustomBody: host.status_code === 200 || !!host.custom_body,
  })
  baseOpenEdit(host)
}

function selectStatus(code: 200 | 404 | 410 | 503) {
  form.status_code = code
  if (code === 200 && !form.useCustomBody) {
    form.useCustomBody = true
    if (!form.custom_body) form.custom_body = starterHtml
  }
}

async function saveForm() {
  if (!form.domain_names.length) {
    saveError.value = 'At least one domain name is required.'
    return
  }
  if (form.status_code === 200 && (!form.useCustomBody || !form.custom_body.trim())) {
    saveError.value = 'A custom HTML page is required when using 200 Custom Page.'
    return
  }
  saving.value = true
  saveError.value = ''
  try {
    const payload: CreateDeadHostInput = {
      domain_names: form.domain_names,
      http_only: form.http_only,
      certificate_id: form.http_only ? null : form.certificate_id,
      access_list_id: form.access_list_id,
      server_id: form.server_id || 1,
      status_code: form.status_code,
      custom_body: form.useCustomBody ? form.custom_body : '',
    }
    if (modalMode.value === 'create') {
      appendItem(await createDeadHost(payload))
    } else if (editingId.value !== null) {
      upsert(await updateDeadHost(editingId.value, payload))
    }
    closeModal()
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(host: DeadHost) {
  try {
    const updated = host.enabled ? await disableDeadHost(host.id) : await enableDeadHost(host.id)
    upsert(updated)
  } catch {
    /* ignore */
  }
}

function statusBadgeClass(code: number): string {
  const map: Record<number, string> = {
    200: 'bg-emerald-100 text-emerald-700',
    404: 'bg-slate-100 text-slate-600',
    410: 'bg-red-100 text-red-700',
    503: 'bg-amber-100 text-amber-700',
  }
  return map[code] ?? 'bg-slate-100 text-slate-600'
}
</script>

<template>
  <div>
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Parked Domains</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          Domains you own but don't forward anywhere — returns an error response instead of timing
          out or matching another rule.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500"
        @click="openCreate"
      >
        + Park Domain
      </button>
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>
    <div
      v-else-if="!loading && hosts.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <p class="text-sm font-medium text-slate-500">No parked domains yet.</p>
      <p class="mt-1 text-xs text-slate-400">
        Park a domain to return a clean error instead of a timeout or accidental proxy match.
      </p>
    </div>
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Domain Names</th>
            <th class="px-4 py-3 font-medium text-slate-600">TLS</th>
            <th class="px-4 py-3 font-medium text-slate-600">Response</th>
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
            <td class="px-4 py-3">
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="d in host.domain_names"
                  :key="d"
                  class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700"
                  >{{ d }}</span
                >
              </div>
            </td>
            <td class="px-4 py-3">
              <TLSStatusBadge
                :status="tlsFor(host.id)"
                :server-id="host.server_id"
                :enabled="host.enabled"
                :https="!host.http_only"
                :load-error="tlsError"
              />
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded px-2 py-0.5 text-xs font-medium"
                :class="statusBadgeClass(host.status_code)"
              >
                {{ host.status_code }}
              </span>
            </td>
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
                {{ host.enabled ? 'Active' : 'Inactive' }}
              </button>
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex items-center justify-end gap-2">
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

    <!-- ── Add / Edit Modal ────────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeModal"
      >
        <div class="flex w-full max-w-lg flex-col rounded-2xl bg-white shadow-2xl">
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">
              {{ modalMode === 'create' ? 'Park a Domain' : 'Edit Parked Domain' }}
            </h3>
            <button class="rounded-lg p-1 text-slate-400 hover:bg-slate-100" @click="closeModal">
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
          <div class="space-y-4 overflow-y-auto px-6 py-5" style="max-height: 65vh">
            <label class="flex items-center gap-2 text-sm"
              ><input v-model="form.http_only" type="checkbox" /> HTTP only (no certificate)</label
            >
            <p v-if="form.http_only" class="text-xs text-slate-500">
              Serve only over HTTP. No HTTPS route or certificate request is created.
            </p>
            <label class="block text-sm"
              >Certificate
              <select
                v-model="form.certificate_id"
                :disabled="form.http_only"
                class="w-full rounded border p-2"
              >
                <option :value="null">Automatic HTTPS</option>
                <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
                  {{ cert.name }}
                </option>
              </select>
            </label>
            <label class="block text-sm"
              >Access list
              <select v-model="form.access_list_id" class="w-full rounded border p-2">
                <option :value="null">Public</option>
                <option v-for="list in accessLists" :key="list.id" :value="list.id">
                  {{ list.name }}
                </option>
              </select>
            </label>
            <div v-if="saveError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ saveError }}
            </div>

            <!-- Info box -->
            <div
              class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-700"
            >
              Any request to these domains will receive the chosen error response. No traffic is
              forwarded anywhere. For a redirect, use
              <span class="font-medium">Redirect Rules</span> instead.
            </div>

            <!-- Domain names -->
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Domain Names</label>
              <div
                class="flex flex-wrap gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500"
              >
                <span
                  v-for="(d, i) in form.domain_names"
                  :key="d"
                  class="flex items-center gap-1 rounded bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700"
                >
                  {{ d
                  }}<button type="button" class="hover:text-indigo-900" @click="removeDomain(i)">
                    &times;
                  </button>
                </span>
                <input
                  v-model="form.domainInput"
                  type="text"
                  placeholder="old-site.com"
                  class="min-w-32 flex-1 border-0 bg-transparent p-0 text-sm focus:outline-none focus:ring-0"
                  @keydown="domainKeydown"
                  @blur="addDomain"
                />
              </div>
              <p class="mt-1 text-xs text-slate-400">Press Enter or comma to add each domain.</p>
            </div>

            <!-- Response type cards -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">Response Type</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="opt in statusOptions"
                  :key="opt.code"
                  type="button"
                  class="rounded-xl border-2 p-3 text-left transition-colors cursor-pointer"
                  :class="
                    form.status_code === opt.code
                      ? 'border-indigo-500 bg-indigo-50'
                      : 'border-slate-200 bg-white hover:border-slate-300'
                  "
                  @click="selectStatus(opt.code)"
                >
                  <div class="text-xs font-semibold text-slate-800">{{ opt.label }}</div>
                  <div class="mt-0.5 text-xs leading-snug text-slate-500">
                    {{ opt.description }}
                  </div>
                </button>
              </div>
            </div>

            <!-- Custom page HTML -->
            <div class="rounded-xl border border-slate-200 bg-slate-50 p-4">
              <label class="flex cursor-pointer items-center gap-2">
                <input
                  v-model="form.useCustomBody"
                  type="checkbox"
                  class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                />
                <span class="text-sm font-medium text-slate-700">
                  {{ form.status_code === 200 ? 'Page HTML' : 'Custom page HTML' }}
                  <span v-if="form.status_code !== 200" class="text-xs font-normal text-slate-400"
                    >(optional — overrides the default error page)</span
                  >
                </span>
              </label>
              <div v-if="form.useCustomBody" class="mt-3">
                <textarea
                  v-model="form.custom_body"
                  rows="12"
                  class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-800 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>
          </div>
          <div class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4">
            <button
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              :disabled="saving"
              @click="closeModal"
            >
              Cancel
            </button>
            <button
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
              :disabled="saving"
              @click="saveForm"
            >
              {{ saving ? 'Saving…' : modalMode === 'create' ? 'Park Domain' : 'Save Changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ── Delete Confirmation ─────────────────────────────────────────────── -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Remove Parked Domain"
      message="This will stop returning an error for these domains. Requests may fall through to other rules or time out. This cannot be undone."
      confirm-label="Remove"
      busy-label="Removing…"
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
