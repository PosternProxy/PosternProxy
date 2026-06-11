<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import type { AuditLogEntry } from '@/types'
import { listAuditLog } from '@/api/auditLog'

const entries = ref<AuditLogEntry[]>([])
const loading = ref(false)
const error = ref('')
const total = ref(0)
const page = ref(1)
const limit = 50
const resourceTypeFilter = ref('')

const resourceTypes = [
  { value: '', label: 'All resources' },
  { value: 'proxy_host', label: 'Proxy Hosts' },
  { value: 'redirection_host', label: 'Redirection Hosts' },
  { value: 'dead_host', label: '404 Hosts' },
  { value: 'stream_host', label: 'Stream Hosts' },
  { value: 'certificate', label: 'Certificates' },
  { value: 'access_list', label: 'Access Lists' },
  { value: 'user', label: 'Users' },
]

onMounted(fetchEntries)
watch([page, resourceTypeFilter], fetchEntries)

async function fetchEntries() {
  loading.value = true
  error.value = ''
  try {
    const result = await listAuditLog({
      resource_type: resourceTypeFilter.value || undefined,
      page: page.value,
      limit,
    })
    entries.value = result.data ?? []
    total.value = result.total
  } catch {
    error.value = 'Failed to load audit log.'
  } finally {
    loading.value = false
  }
}

function resetFilter() {
  page.value = 1
  resourceTypeFilter.value = ''
}

const totalPages = () => Math.max(1, Math.ceil(total.value / limit))

function actionBadgeClass(action: string) {
  return (
    {
      create: 'bg-emerald-100 text-emerald-700',
      update: 'bg-blue-100 text-blue-700',
      delete: 'bg-red-100 text-red-700',
      enable: 'bg-emerald-100 text-emerald-700',
      disable: 'bg-slate-100 text-slate-600',
      login: 'bg-indigo-100 text-indigo-700',
      logout: 'bg-slate-100 text-slate-600',
    }[action] ?? 'bg-slate-100 text-slate-600'
  )
}

function formatTime(ts: string) {
  try {
    return new Date(ts).toLocaleString()
  } catch {
    return ts
  }
}

function resourceLabel(rt: string) {
  return rt.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}
</script>

<template>
  <div>
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Audit Log</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          All configuration changes and authentication events
        </p>
      </div>
      <div class="flex items-center gap-2">
        <select
          v-model="resourceTypeFilter"
          class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          @change="page = 1"
        >
          <option v-for="rt in resourceTypes" :key="rt.value" :value="rt.value">
            {{ rt.label }}
          </option>
        </select>
        <button
          v-if="resourceTypeFilter"
          class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-600 hover:bg-slate-50"
          @click="resetFilter"
        >
          Clear
        </button>
      </div>
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 8" :key="i" class="h-12 animate-pulse rounded-xl bg-slate-100" />
    </div>
    <div
      v-else-if="!loading && entries.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <p class="text-sm font-medium text-slate-500">No audit log entries yet.</p>
    </div>
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Time</th>
            <th class="px-4 py-3 font-medium text-slate-600">Action</th>
            <th class="px-4 py-3 font-medium text-slate-600">Resource</th>
            <th class="px-4 py-3 font-medium text-slate-600">Details</th>
            <th class="px-4 py-3 font-medium text-slate-600">IP Address</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="entry in entries" :key="entry.id" class="hover:bg-slate-50">
            <td class="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">
              {{ formatTime(entry.created_at) }}
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium capitalize"
                :class="actionBadgeClass(entry.action)"
              >
                {{ entry.action }}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-slate-600">
              {{ resourceLabel(entry.resource_type) }}
              <span v-if="entry.resource_id" class="ml-1 text-xs text-slate-400"
                >#{{ entry.resource_id }}</span
              >
            </td>
            <td class="px-4 py-3 text-xs text-slate-500 font-mono max-w-xs truncate">
              {{
                typeof entry.details === 'object' ? JSON.stringify(entry.details) : entry.details
              }}
            </td>
            <td class="px-4 py-3 text-xs text-slate-500 font-mono">
              {{ entry.ip_address ?? '—' }}
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div
        v-if="totalPages() > 1"
        class="flex items-center justify-between border-t border-slate-200 px-4 py-3"
      >
        <p class="text-xs text-slate-500">
          Showing {{ (page - 1) * limit + 1 }}–{{ Math.min(page * limit, total) }} of
          {{ total }} entries
        </p>
        <div class="flex gap-1">
          <button
            class="rounded px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 disabled:opacity-40"
            :disabled="page <= 1"
            @click="page--"
          >
            ← Prev
          </button>
          <span class="px-2 py-1 text-xs text-slate-500">{{ page }} / {{ totalPages() }}</span>
          <button
            class="rounded px-2 py-1 text-xs text-slate-600 hover:bg-slate-100 disabled:opacity-40"
            :disabled="page >= totalPages()"
            @click="page++"
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
