<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getSettings, updateSettings } from '@/api/settings'
import { errorMessage } from '@/utils/errors'

const loading = ref(true)
const saving = ref(false)
const error = ref('')
const success = ref('')

// Editable fields
const caddyAPIURL = ref('')
const certStoragePath = ref('')

async function fetchSettings() {
  loading.value = true
  error.value = ''
  try {
    const s = await getSettings()
    caddyAPIURL.value = s['caddy_api_url'] ?? ''
    certStoragePath.value = s['cert_storage_path'] ?? ''
  } catch (e: unknown) {
    error.value = errorMessage(e, 'Failed to load settings')
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    await updateSettings({
      caddy_api_url: caddyAPIURL.value.trim(),
      cert_storage_path: certStoragePath.value.trim(),
    })
    success.value = 'Settings saved successfully.'
  } catch (e: unknown) {
    error.value = errorMessage(e, 'Failed to save settings')
  } finally {
    saving.value = false
  }
}

onMounted(fetchSettings)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold text-slate-800 mb-6">Settings</h2>

    <!-- Loading skeleton -->
    <div
      v-if="loading"
      class="rounded-xl border border-slate-200 bg-white p-8 shadow-sm animate-pulse"
    >
      <div class="h-4 w-1/3 rounded bg-slate-200 mb-6"></div>
      <div class="space-y-4">
        <div class="h-9 rounded bg-slate-100"></div>
        <div class="h-9 rounded bg-slate-100"></div>
      </div>
    </div>

    <div v-else class="space-y-6">
      <!-- Error banner -->
      <div
        v-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
      >
        {{ error }}
      </div>

      <!-- Success banner -->
      <div
        v-if="success"
        class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
      >
        {{ success }}
      </div>

      <!-- Caddy Settings -->
      <div class="rounded-xl border border-slate-200 bg-white shadow-sm">
        <div class="border-b border-slate-100 px-6 py-4">
          <h3 class="font-semibold text-slate-800">Caddy</h3>
          <p class="mt-0.5 text-sm text-slate-500">
            Configure how PosternProxy connects to the local Caddy instance.
          </p>
        </div>
        <div class="px-6 py-5 space-y-4">
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1">
              Caddy Admin API URL
            </label>
            <input
              v-model="caddyAPIURL"
              type="text"
              placeholder="http://localhost:2019"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
            />
            <p class="mt-1 text-xs text-slate-400">
              The URL of the Caddy admin API. Should only be accessible from localhost.
            </p>
          </div>
        </div>
      </div>

      <!-- Certificates Settings -->
      <div class="rounded-xl border border-slate-200 bg-white shadow-sm">
        <div class="border-b border-slate-100 px-6 py-4">
          <h3 class="font-semibold text-slate-800">Certificates</h3>
          <p class="mt-0.5 text-sm text-slate-500">Storage path for uploaded certificate files.</p>
        </div>
        <div class="px-6 py-5 space-y-4">
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1">
              Certificate Storage Path
            </label>
            <input
              v-model="certStoragePath"
              type="text"
              placeholder="data/certs"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
            />
            <p class="mt-1 text-xs text-slate-400">
              Absolute or relative path where uploaded certificate files are stored on disk.
            </p>
          </div>
        </div>
      </div>

      <!-- Save button -->
      <div class="flex justify-end">
        <button
          @click="save"
          :disabled="saving"
          class="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-5 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors"
        >
          <svg v-if="saving" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
          </svg>
          {{ saving ? 'Saving…' : 'Save Settings' }}
        </button>
      </div>
    </div>
  </div>
</template>
