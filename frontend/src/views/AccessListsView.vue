<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import type { AccessList, AccessListClient } from '@/types'
import {
  listAccessLists,
  createAccessList,
  updateAccessList,
  deleteAccessList,
  type CreateAccessListInput,
} from '@/api/accessLists'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'
import { useResourceList } from '@/composables/useResourceList'

const {
  items: lists,
  loading,
  loadError: error,
  load: fetchLists,
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
} = useResourceList<AccessList>({
  list: listAccessLists,
  delete: deleteAccessList,
  itemNoun: 'access list',
})

interface ClientRow {
  address: string
  directive: 'allow' | 'deny'
  sort_order: number
}

const defaultForm = () => ({
  name: '',
  clients: [] as ClientRow[],
})
const form = reactive(defaultForm())

onMounted(fetchLists)

function openCreate() {
  Object.assign(form, defaultForm())
  baseOpenCreate()
}
function openEdit(al: AccessList) {
  Object.assign(form, {
    name: al.name,
    clients: al.clients
      ? al.clients.map((c: AccessListClient) => ({
          address: c.address,
          directive: c.directive,
          sort_order: c.sort_order,
        }))
      : [],
  })
  baseOpenEdit(al)
}

function addClient() {
  form.clients.push({ address: '', directive: 'allow', sort_order: form.clients.length })
}
function removeClient(i: number) {
  form.clients.splice(i, 1)
}

async function saveForm() {
  if (!form.name.trim()) {
    saveError.value = 'Name is required.'
    return
  }
  saving.value = true
  saveError.value = ''
  try {
    const payload: CreateAccessListInput = {
      name: form.name.trim(),
      clients: form.clients.filter((c) => c.address.trim()),
    }
    if (modalMode.value === 'create') {
      appendItem(await createAccessList(payload))
    } else if (editingId.value !== null) {
      upsert(await updateAccessList(editingId.value, payload))
    }
    closeModal()
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div>
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Access Lists</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          Named IP allowlists and blocklists you can apply to proxy hosts.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500"
        @click="openCreate"
      >
        + Add Access List
      </button>
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>
    <div
      v-else-if="!loading && lists.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <p class="text-sm font-medium text-slate-500">No access lists configured yet.</p>
      <p class="mt-1 text-xs text-slate-400">
        Create an access list to restrict proxy hosts by IP address or CIDR range.
      </p>
    </div>
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Name</th>
            <th class="px-4 py-3 font-medium text-slate-600">IP Rules</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="al in lists" :key="al.id" class="transition-colors hover:bg-slate-50">
            <td class="px-4 py-3 font-medium text-slate-800">{{ al.name }}</td>
            <td class="px-4 py-3 text-sm text-slate-600">
              <span v-if="al.clients && al.clients.length">
                {{ al.clients.length }} rule{{ al.clients.length !== 1 ? 's' : '' }}
              </span>
              <span v-else class="text-slate-400">None</span>
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex items-center justify-end gap-2">
                <button
                  class="rounded px-2 py-1 text-xs text-indigo-600 hover:bg-indigo-50"
                  @click="openEdit(al)"
                >
                  Edit
                </button>
                <button
                  class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
                  @click="confirmDelete(al.id)"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Modal -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeModal"
      >
        <div class="flex w-full max-w-xl flex-col rounded-2xl bg-white shadow-2xl">
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">
              {{ modalMode === 'create' ? 'Add Access List' : 'Edit Access List' }}
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

          <div class="space-y-4 overflow-y-auto px-6 py-5" style="max-height: 60vh">
            <div v-if="saveError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ saveError }}
            </div>

            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="Office Network"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">IP Rules</label>
              <div v-if="form.clients.length === 0" class="mb-2 text-sm text-slate-400">
                No rules yet — without rules all traffic is allowed.
              </div>
              <div class="space-y-2">
                <div v-for="(client, i) in form.clients" :key="i" class="flex items-center gap-2">
                  <select
                    v-model="client.directive"
                    class="w-24 rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <option value="allow">Allow</option>
                    <option value="deny">Deny</option>
                  </select>
                  <input
                    v-model="client.address"
                    type="text"
                    placeholder="192.168.1.0/24"
                    class="flex-1 rounded-lg border border-slate-300 px-3 py-1.5 font-mono text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                  <button
                    class="rounded p-1 text-slate-400 hover:text-red-500"
                    @click="removeClient(i)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M6 18L18 6M6 6l12 12"
                      />
                    </svg>
                  </button>
                </div>
              </div>
              <button
                class="mt-3 flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800"
                @click="addClient"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                Add Rule
              </button>
              <p class="mt-3 text-xs text-slate-400">
                Deny rules block matching IPs first. Allow rules create an allowlist — everything
                else is blocked. You can mix both: denied IPs are blocked even if they appear in the
                allow list.
              </p>
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
              {{ saving ? 'Saving…' : modalMode === 'create' ? 'Create' : 'Save Changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Delete Access List"
      message="Any proxy hosts using this access list will lose their IP restrictions. This cannot be undone."
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
