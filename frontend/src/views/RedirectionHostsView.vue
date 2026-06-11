<script setup lang="ts">
import { reactive, onMounted, watch } from 'vue'
import type { RedirectionHost } from '@/types'
import {
  listRedirectionHosts,
  createRedirectionHost,
  updateRedirectionHost,
  deleteRedirectionHost,
  enableRedirectionHost,
  disableRedirectionHost,
  type CreateRedirectionHostInput,
} from '@/api/redirectionHosts'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'

const serverStore = useServerStore()
const ctx = useServerContext()

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
} = useResourceList<RedirectionHost>({
  list: () => listRedirectionHosts(ctx.selectedServerId),
  delete: deleteRedirectionHost,
  itemNoun: 'redirection',
})

const defaultForm = () => ({
  domainInput: '',
  domain_names: [] as string[],
  server_id: 1,
  forward_scheme: 'https' as 'auto' | 'http' | 'https',
  forward_domain_name: '',
  forward_path: '',
  preserve_path: true,
  status_code: 301 as 301 | 302 | 307 | 308,
})

const form = reactive(defaultForm())

onMounted(async () => {
  await serverStore.ensureLoaded()
  await fetchHosts()
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

function openEdit(host: RedirectionHost) {
  Object.assign(form, {
    domainInput: '',
    domain_names: [...host.domain_names],
    server_id: host.server_id ?? 1,
    forward_scheme: host.forward_scheme,
    forward_domain_name: host.forward_domain_name,
    forward_path: host.forward_path ?? '',
    preserve_path: host.preserve_path,
    status_code: host.status_code,
  })
  baseOpenEdit(host)
}

async function saveForm() {
  if (!form.domain_names.length) {
    saveError.value = 'At least one source domain is required.'
    return
  }
  if (!form.forward_domain_name) {
    saveError.value = 'Destination domain is required.'
    return
  }
  saving.value = true
  saveError.value = ''
  try {
    const payload: CreateRedirectionHostInput = {
      domain_names: form.domain_names,
      server_id: form.server_id || 1,
      forward_scheme: form.forward_scheme,
      forward_domain_name: form.forward_domain_name,
      forward_path: form.forward_path || undefined,
      preserve_path: form.preserve_path,
      status_code: form.status_code,
    }
    if (modalMode.value === 'create') {
      appendItem(await createRedirectionHost(payload))
    } else if (editingId.value !== null) {
      upsert(await updateRedirectionHost(editingId.value, payload))
    }
    closeModal()
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(host: RedirectionHost) {
  try {
    const updated = host.enabled
      ? await disableRedirectionHost(host.id)
      : await enableRedirectionHost(host.id)
    upsert(updated)
  } catch {
    /* ignore */
  }
}

const redirectTypeOptions = [
  {
    value: 301,
    label: 'Permanent',
    code: '301',
    desc: 'Browser caches this redirect. Use when the old domain is gone for good.',
  },
  {
    value: 302,
    label: 'Temporary',
    code: '302',
    desc: 'Browser re-checks every time. Use when you may revert this redirect.',
  },
] as const
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Redirect Rules</h2>
        <p class="mt-0.5 text-sm text-slate-500">Send browsers from one domain to another</p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="openCreate"
      >
        + Add Redirect Rule
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
      v-else-if="hosts.length === 0"
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
          d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
      <p class="text-sm font-medium text-slate-500">No redirect rules configured yet.</p>
      <p class="mt-1 text-xs text-slate-400">Click "Add Redirect Rule" to get started.</p>
    </div>

    <!-- Table -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Source Domains</th>
            <th class="px-4 py-3 font-medium text-slate-600">Redirects To</th>
            <th class="px-4 py-3 font-medium text-slate-600">Type</th>
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
            <td class="px-4 py-3 font-mono text-xs text-slate-600">
              {{ host.forward_scheme === 'auto' ? 'https' : host.forward_scheme }}://{{
                host.forward_domain_name
              }}{{ host.forward_path }}
              <span v-if="host.preserve_path" class="ml-1 font-sans text-slate-400">+ path</span>
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-semibold"
                :class="
                  host.status_code === 301 || host.status_code === 308
                    ? 'bg-violet-100 text-violet-700'
                    : 'bg-amber-100 text-amber-700'
                "
              >
                {{
                  host.status_code === 301
                    ? '301 Permanent'
                    : host.status_code === 302
                      ? '302 Temporary'
                      : host.status_code
                }}
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
                {{ host.enabled ? 'Active' : 'Paused' }}
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

    <!-- ── Add / Edit Modal ──────────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeModal"
      >
        <div class="flex w-full max-w-lg flex-col rounded-2xl bg-white shadow-2xl">
          <!-- Header -->
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">
              {{ modalMode === 'create' ? 'Add Redirect Rule' : 'Edit Redirect Rule' }}
            </h3>
            <button
              class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
              @click="closeModal"
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

          <!-- Body -->
          <div class="space-y-5 overflow-y-auto px-6 py-5" style="max-height: 70vh">
            <!-- What this is -->
            <div
              class="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-xs text-blue-700"
            >
              <p class="font-semibold">What this does</p>
              <p class="mt-1">
                Tells the browser to go to a different URL. No content is served — Caddy returns an
                HTTP redirect code and the browser navigates on its own.
              </p>
              <p class="mt-1 text-blue-500">
                For SSL, access control, or custom headers, use a Proxy Host instead.
              </p>
            </div>

            <!-- Error -->
            <div v-if="saveError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ saveError }}
            </div>

            <!-- Source domains -->
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Source Domains</label>
              <div
                class="flex flex-wrap gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500"
              >
                <span
                  v-for="(d, i) in form.domain_names"
                  :key="d"
                  class="flex items-center gap-1 rounded bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700"
                >
                  {{ d }}
                  <button type="button" class="hover:text-indigo-900" @click="removeDomain(i)">
                    &times;
                  </button>
                </span>
                <input
                  v-model="form.domainInput"
                  type="text"
                  placeholder="old-domain.com"
                  class="min-w-32 flex-1 border-0 bg-transparent p-0 text-sm focus:outline-none focus:ring-0"
                  @keydown="domainKeydown"
                  @blur="addDomain"
                />
              </div>
              <p class="mt-1 text-xs text-slate-400">Press Enter or comma to add each domain</p>
            </div>

            <!-- Destination -->
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Destination</label>
              <div class="flex gap-2">
                <select
                  v-model="form.forward_scheme"
                  class="w-24 shrink-0 rounded-lg border border-slate-300 bg-white px-2 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                >
                  <option value="https">https://</option>
                  <option value="http">http://</option>
                  <option value="auto">auto</option>
                </select>
                <input
                  v-model="form.forward_domain_name"
                  type="text"
                  placeholder="new-domain.com"
                  class="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div class="mt-2">
                <input
                  v-model="form.forward_path"
                  type="text"
                  placeholder="/optional-path-prefix"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
                <p class="mt-1 text-xs text-slate-400">
                  Optional path appended before the request URI
                </p>
              </div>
            </div>

            <!-- Redirect type -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">Redirect Type</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="opt in redirectTypeOptions"
                  :key="opt.value"
                  type="button"
                  class="rounded-lg border px-4 py-3 text-left transition-colors"
                  :class="
                    form.status_code === opt.value
                      ? 'border-indigo-500 bg-indigo-50'
                      : 'border-slate-200 bg-white hover:border-slate-300'
                  "
                  @click="form.status_code = opt.value"
                >
                  <div class="flex items-center gap-2">
                    <span
                      class="rounded px-1.5 py-0.5 text-xs font-bold"
                      :class="
                        form.status_code === opt.value
                          ? 'bg-indigo-100 text-indigo-700'
                          : 'bg-slate-100 text-slate-600'
                      "
                      >{{ opt.code }}</span
                    >
                    <span
                      class="text-sm font-medium"
                      :class="form.status_code === opt.value ? 'text-indigo-800' : 'text-slate-700'"
                    >
                      {{ opt.label }}
                    </span>
                  </div>
                  <p
                    class="mt-1.5 text-xs"
                    :class="form.status_code === opt.value ? 'text-indigo-600' : 'text-slate-400'"
                  >
                    {{ opt.desc }}
                  </p>
                </button>
              </div>
              <p
                v-if="form.status_code === 307 || form.status_code === 308"
                class="mt-2 text-xs text-slate-400"
              >
                307/308 preserve the HTTP method (POST stays POST). Use 301/302 for standard browser
                redirects.
              </p>
            </div>

            <!-- Preserve path -->
            <label
              class="flex cursor-pointer items-start gap-3 rounded-lg border border-slate-200 p-4"
            >
              <input
                v-model="form.preserve_path"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
              />
              <div>
                <p class="text-sm font-medium text-slate-800">Preserve Path</p>
                <p class="mt-0.5 text-xs text-slate-500">
                  Append the original request path to the destination. e.g.
                  <span class="font-mono">old.com/blog/post</span> →
                  <span class="font-mono">new.com/blog/post</span>
                </p>
              </div>
            </label>
          </div>

          <!-- Footer -->
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
              {{ saving ? 'Saving…' : modalMode === 'create' ? 'Create' : 'Save Changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ── Delete Confirmation ───────────────────────────────────────────────── -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Delete Redirect Rule"
      message="Traffic to the source domains will no longer be redirected. This cannot be undone."
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
