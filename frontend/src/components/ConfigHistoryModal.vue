<script setup lang="ts">
import { ref, watch } from 'vue'
import type { ConfigHistory } from '@/types'
import { listHistory, rollbackHistory } from '@/api/configHistory'

const props = defineProps<{
  resourceType: string
  resourceId: number
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'rolledBack'): void
}>()

const history = ref<ConfigHistory[]>([])
const loading = ref(false)
const error = ref('')

// Rollback confirm state
const confirmId = ref<number | null>(null)
const rollingBack = ref(false)
const rollbackError = ref('')

watch(
  () => props.show,
  async (shown) => {
    if (shown) {
      await fetchHistory()
    }
  }
)

async function fetchHistory() {
  loading.value = true
  error.value = ''
  history.value = []
  try {
    history.value = await listHistory(props.resourceType, props.resourceId)
  } catch {
    error.value = 'Failed to load configuration history.'
  } finally {
    loading.value = false
  }
}

async function doRollback() {
  if (confirmId.value === null) return
  rollingBack.value = true
  rollbackError.value = ''
  try {
    await rollbackHistory(confirmId.value)
    confirmId.value = null
    emit('rolledBack')
    emit('close')
  } catch {
    rollbackError.value = 'Rollback failed. Please try again.'
  } finally {
    rollingBack.value = false
  }
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString()
}

function actionLabel(action: string) {
  const map: Record<string, string> = {
    create: 'Created',
    update: 'Updated',
    delete: 'Deleted',
    enable: 'Enabled',
    disable: 'Disabled',
  }
  return map[action] ?? action
}

function actionColor(action: string) {
  const map: Record<string, string> = {
    create: 'bg-emerald-100 text-emerald-700',
    update: 'bg-indigo-100 text-indigo-700',
    delete: 'bg-red-100 text-red-700',
    enable: 'bg-green-100 text-green-700',
    disable: 'bg-slate-100 text-slate-600',
  }
  return map[action] ?? 'bg-slate-100 text-slate-600'
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="$emit('close')"
    >
      <div class="flex w-full max-w-xl flex-col rounded-2xl bg-white shadow-2xl">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h3 class="text-base font-semibold text-slate-800">Configuration History</h3>
          <button
            class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            @click="$emit('close')"
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
        <div class="flex-1 overflow-y-auto px-6 py-5" style="max-height: 60vh">
          <!-- Error -->
          <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
            {{ error }}
          </div>

          <!-- Rollback error -->
          <div v-if="rollbackError" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
            {{ rollbackError }}
          </div>

          <!-- Loading skeleton -->
          <div v-if="loading" class="space-y-3">
            <div v-for="i in 4" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
          </div>

          <!-- Empty state -->
          <div
            v-else-if="!loading && history.length === 0"
            class="rounded-xl border border-dashed border-slate-300 p-10 text-center"
          >
            <svg
              class="mx-auto mb-2 h-8 w-8 text-slate-300"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.5"
                d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p class="text-sm text-slate-500">No configuration history found.</p>
          </div>

          <!-- Timeline -->
          <ol v-else class="relative border-l border-slate-200">
            <li v-for="entry in history" :key="entry.id" class="mb-6 ml-4 last:mb-0">
              <!-- Dot -->
              <div
                class="absolute -left-1.5 mt-1.5 h-3 w-3 rounded-full border-2 border-white bg-slate-300"
              />

              <div class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
                <div class="mb-1 flex items-center justify-between gap-2">
                  <span
                    class="rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="actionColor(entry.action)"
                  >
                    {{ actionLabel(entry.action) }}
                  </span>
                  <span class="text-xs text-slate-400">{{ formatDate(entry.created_at) }}</span>
                </div>
                <p class="text-xs text-slate-500">
                  Entry #{{ entry.id }}
                  <span v-if="entry.created_by"> &mdash; by user {{ entry.created_by }}</span>
                </p>

                <!-- Rollback button -->
                <div class="mt-3 flex items-center justify-end">
                  <template v-if="confirmId === entry.id">
                    <span class="mr-3 text-xs text-slate-600">Roll back to this snapshot?</span>
                    <button
                      class="mr-2 rounded border border-slate-300 px-3 py-1 text-xs text-slate-700 hover:bg-slate-50"
                      :disabled="rollingBack"
                      @click="confirmId = null"
                    >
                      Cancel
                    </button>
                    <button
                      class="rounded bg-amber-600 px-3 py-1 text-xs font-medium text-white hover:bg-amber-500 disabled:opacity-50"
                      :disabled="rollingBack"
                      @click="doRollback"
                    >
                      {{ rollingBack ? 'Rolling back…' : 'Confirm Rollback' }}
                    </button>
                  </template>
                  <button
                    v-else
                    class="rounded border border-amber-300 px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-50"
                    @click="confirmId = entry.id"
                  >
                    Rollback
                  </button>
                </div>
              </div>
            </li>
          </ol>
        </div>

        <!-- Footer -->
        <div class="flex justify-end border-t border-slate-200 px-6 py-4">
          <button
            class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            @click="$emit('close')"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
