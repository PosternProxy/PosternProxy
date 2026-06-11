<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import type { Certificate } from '@/types'
import { listCertificates, uploadCustomCertificate, deleteCertificate } from '@/api/certificates'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'

const {
  items: certs,
  loading,
  loadError: error,
  load: fetchCerts,
  showModal,
  saving,
  saveError,
  openCreate: baseOpenCreate,
  closeModal,
  showDeleteConfirm,
  deleting,
  deleteError,
  confirmDelete,
  doDelete,
  appendItem,
} = useResourceList<Certificate>({
  list: listCertificates,
  delete: deleteCertificate,
  itemNoun: 'certificate',
})

const domainInput = ref('')
const domainNames = ref<string[]>([])

const form = reactive({
  name: '',
  certFile: null as File | null,
  keyFile: null as File | null,
})

onMounted(fetchCerts)

function addDomain() {
  const v = domainInput.value.trim()
  if (v && !domainNames.value.includes(v)) domainNames.value.push(v)
  domainInput.value = ''
}
function removeDomain(i: number) {
  domainNames.value.splice(i, 1)
}
function domainKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    addDomain()
  } else if (e.key === 'Backspace' && domainInput.value === '' && domainNames.value.length)
    domainNames.value.pop()
}

function openCreate() {
  domainInput.value = ''
  domainNames.value = []
  Object.assign(form, { name: '', certFile: null, keyFile: null })
  baseOpenCreate()
}

async function saveForm() {
  if (!domainNames.value.length) {
    saveError.value = 'At least one domain name is required.'
    return
  }
  if (!form.name) {
    saveError.value = 'Name is required.'
    return
  }
  if (!form.certFile) {
    saveError.value = 'Certificate file is required.'
    return
  }
  if (!form.keyFile) {
    saveError.value = 'Private key file is required.'
    return
  }
  saving.value = true
  saveError.value = ''
  try {
    const cert = await uploadCustomCertificate(
      form.name,
      domainNames.value,
      form.certFile,
      form.keyFile
    )
    appendItem(cert)
    closeModal()
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : 'Upload failed'
  } finally {
    saving.value = false
  }
}

function statusClass(s: string) {
  return (
    {
      active: 'bg-emerald-100 text-emerald-700',
      pending: 'bg-amber-100 text-amber-700',
      expired: 'bg-red-100 text-red-700',
      error: 'bg-red-100 text-red-700',
    }[s] ?? 'bg-slate-100 text-slate-600'
  )
}

function daysUntilExpiry(expiresAt: string | undefined) {
  if (!expiresAt) return null
  return Math.ceil((new Date(expiresAt).getTime() - Date.now()) / 86400000)
}

const expiryClass = computed(() => (cert: Certificate) => {
  const d = daysUntilExpiry(cert.expires_at ?? undefined)
  if (d === null) return 'text-slate-400'
  if (d <= 7) return 'text-red-600 font-medium'
  if (d <= 30) return 'text-amber-600'
  return 'text-slate-600'
})
</script>

<template>
  <div>
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Custom Certificates</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          Upload PEM certificates from your internal CA or a commercial provider.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500"
        @click="openCreate"
      >
        + Upload Certificate
      </button>
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>
    <div
      v-else-if="!loading && certs.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <p class="text-sm font-medium text-slate-500">No custom certificates uploaded yet.</p>
      <p class="mt-1 text-xs text-slate-400">
        Caddy manages Let's Encrypt automatically. Upload here only for internal CA or commercial
        certificates.
      </p>
    </div>
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Name</th>
            <th class="px-4 py-3 font-medium text-slate-600">Domains</th>
            <th class="px-4 py-3 font-medium text-slate-600">Expires</th>
            <th class="px-4 py-3 font-medium text-slate-600">Status</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="cert in certs" :key="cert.id" class="transition-colors hover:bg-slate-50">
            <td class="px-4 py-3 font-medium text-slate-800">{{ cert.name }}</td>
            <td class="px-4 py-3">
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="d in cert.domain_names"
                  :key="d"
                  class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700"
                  >{{ d }}</span
                >
              </div>
            </td>
            <td class="px-4 py-3 text-xs" :class="expiryClass(cert)">
              <template v-if="cert.expires_at">
                {{ cert.expires_at.slice(0, 10) }}
                <span class="ml-1 text-slate-400">({{ daysUntilExpiry(cert.expires_at) }}d)</span>
              </template>
              <span v-else class="text-slate-400">—</span>
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium capitalize"
                :class="statusClass(cert.status)"
              >
                {{ cert.status }}
              </span>
            </td>
            <td class="px-4 py-3 text-right">
              <button
                class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
                @click="confirmDelete(cert.id)"
              >
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Upload Modal -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeModal"
      >
        <div class="flex w-full max-w-xl flex-col rounded-2xl bg-white shadow-2xl">
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">Upload Certificate</h3>
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

          <div class="space-y-4 overflow-y-auto px-6 py-5" style="max-height: 60vh">
            <div v-if="saveError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ saveError }}
            </div>

            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="Internal CA Wildcard"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Domain Names</label>
              <div
                class="flex flex-wrap gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500"
              >
                <span
                  v-for="(d, i) in domainNames"
                  :key="d"
                  class="flex items-center gap-1 rounded bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700"
                >
                  {{ d
                  }}<button type="button" class="hover:text-indigo-900" @click="removeDomain(i)">
                    &times;
                  </button>
                </span>
                <input
                  v-model="domainInput"
                  type="text"
                  placeholder="example.com"
                  class="min-w-32 flex-1 border-0 bg-transparent p-0 text-sm focus:outline-none focus:ring-0"
                  @keydown="domainKeydown"
                  @blur="addDomain"
                />
              </div>
              <p class="mt-1 text-xs text-slate-400">Press Enter or comma to add each domain.</p>
            </div>

            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700"
                >Certificate File (.pem / .crt)</label
              >
              <input
                type="file"
                accept=".pem,.crt,.cer"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm file:mr-3 file:rounded file:border-0 file:bg-indigo-50 file:px-3 file:py-1 file:text-sm file:font-medium file:text-indigo-700"
                @change="form.certFile = ($event.target as HTMLInputElement).files?.[0] ?? null"
              />
            </div>

            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700"
                >Private Key File (.pem / .key)</label
              >
              <input
                type="file"
                accept=".pem,.key"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm file:mr-3 file:rounded file:border-0 file:bg-indigo-50 file:px-3 file:py-1 file:text-sm file:font-medium file:text-indigo-700"
                @change="form.keyFile = ($event.target as HTMLInputElement).files?.[0] ?? null"
              />
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
              {{ saving ? 'Uploading…' : 'Upload Certificate' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Delete confirm -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Delete Certificate"
      message="Any proxy hosts using this certificate will fall back to automatic Let's Encrypt. Certificate files will be removed from disk. This cannot be undone."
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
