<script setup lang="ts">
import { ref } from 'vue'
import type { ParseResult, ImportSummary } from '@/types'
import { previewNginxImport, confirmNginxImport } from '@/api/nginxImport'

type Step = 'upload' | 'preview' | 'done'

const step = ref<Step>('upload')
const dragging = ref(false)
const uploading = ref(false)
const uploadError = ref('')

const parseResult = ref<ParseResult | null>(null)
// uploadedFile is kept so we can re-submit the original nginx config at
// Confirm time. The server re-parses it instead of trusting the previewed
// ParseResult.
const uploadedFile = ref<File | null>(null)
const importSummary = ref<ImportSummary | null>(null)
const confirming = ref(false)
const confirmError = ref('')

// ── Drag & drop ───────────────────────────────────────────────────────────────

function onDragOver(e: DragEvent) {
  e.preventDefault()
  dragging.value = true
}

function onDragLeave() {
  dragging.value = false
}

async function onDrop(e: DragEvent) {
  e.preventDefault()
  dragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file) await uploadFile(file)
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) uploadFile(file)
}

// ── Step 1: preview ──────────────────────────────────────────────────────────

async function uploadFile(file: File) {
  uploading.value = true
  uploadError.value = ''
  try {
    parseResult.value = await previewNginxImport(file)
    uploadedFile.value = file
    step.value = 'preview'
  } catch (e: unknown) {
    uploadError.value =
      e instanceof Error ? e.message : 'Failed to parse the nginx configuration file.'
  } finally {
    uploading.value = false
  }
}

// ── Step 2: confirm ───────────────────────────────────────────────────────────

async function doConfirm() {
  if (!uploadedFile.value) return
  confirming.value = true
  confirmError.value = ''
  try {
    importSummary.value = await confirmNginxImport(uploadedFile.value)
    step.value = 'done'
  } catch (e: unknown) {
    confirmError.value = e instanceof Error ? e.message : 'Import failed. Please try again.'
  } finally {
    confirming.value = false
  }
}

function reset() {
  step.value = 'upload'
  parseResult.value = null
  uploadedFile.value = null
  importSummary.value = null
  uploadError.value = ''
  confirmError.value = ''
  const input = document.getElementById('nginx-file-input') as HTMLInputElement | null
  if (input) input.value = ''
}
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6">
    <!-- Page header -->
    <div>
      <h2 class="text-xl font-semibold text-slate-800">Import from Nginx</h2>
      <p class="mt-0.5 text-sm text-slate-500">
        Upload an nginx configuration file to preview the parsed hosts, then confirm the import.
      </p>
    </div>

    <!-- Step indicators -->
    <div class="flex items-center gap-2 text-xs font-medium">
      <span
        class="rounded-full px-3 py-1"
        :class="step === 'upload' ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-500'"
      >
        1. Upload
      </span>
      <span class="text-slate-300">/</span>
      <span
        class="rounded-full px-3 py-1"
        :class="step === 'preview' ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-500'"
      >
        2. Preview
      </span>
      <span class="text-slate-300">/</span>
      <span
        class="rounded-full px-3 py-1"
        :class="step === 'done' ? 'bg-indigo-600 text-white' : 'bg-slate-100 text-slate-500'"
      >
        3. Done
      </span>
    </div>

    <!-- ── Step 1: Upload ─────────────────────────────────────────────────── -->
    <div v-if="step === 'upload'" class="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <div
        class="flex flex-col items-center justify-center rounded-xl border-2 border-dashed px-8 py-14 transition-colors"
        :class="dragging ? 'border-indigo-500 bg-indigo-50' : 'border-slate-300 bg-slate-50'"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <svg
          class="mb-4 h-10 w-10"
          :class="dragging ? 'text-indigo-400' : 'text-slate-300'"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
          />
        </svg>
        <p class="mb-1 text-sm font-medium text-slate-700">
          Drag &amp; drop your nginx config here
        </p>
        <p class="mb-4 text-xs text-slate-400">or click to browse</p>
        <label class="cursor-pointer">
          <span
            class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
          >
            {{ uploading ? 'Parsing…' : 'Choose File' }}
          </span>
          <input
            id="nginx-file-input"
            type="file"
            accept=".conf,.nginx,text/plain"
            class="hidden"
            :disabled="uploading"
            @change="onFileChange"
          />
        </label>
      </div>

      <div v-if="uploadError" class="mt-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
        {{ uploadError }}
      </div>
    </div>

    <!-- ── Step 2: Preview ────────────────────────────────────────────────── -->
    <div v-if="step === 'preview' && parseResult" class="space-y-5">
      <!-- Warnings -->
      <div v-if="parseResult.warnings.length" class="rounded-lg bg-amber-50 p-4">
        <p class="mb-2 text-xs font-semibold text-amber-700">Parse Warnings</p>
        <ul class="list-inside list-disc space-y-1 text-xs text-amber-700">
          <li v-for="(w, i) in parseResult.warnings" :key="i">{{ w }}</li>
        </ul>
      </div>

      <!-- Proxy hosts table -->
      <div class="rounded-xl border border-slate-200 bg-white shadow-sm">
        <div class="border-b border-slate-200 px-5 py-3">
          <h3 class="text-sm font-semibold text-slate-800">
            Proxy Hosts
            <span class="ml-1 text-xs font-normal text-slate-400"
              >({{ parseResult.proxy_hosts.length }})</span
            >
          </h3>
        </div>
        <div
          v-if="parseResult.proxy_hosts.length === 0"
          class="px-5 py-6 text-center text-sm text-slate-400"
        >
          No proxy hosts found.
        </div>
        <table v-else class="w-full text-left text-sm">
          <thead class="border-b border-slate-100 bg-slate-50 text-xs font-medium text-slate-500">
            <tr>
              <th class="px-5 py-2">Domains</th>
              <th class="px-5 py-2">Upstream</th>
              <th class="px-5 py-2">SSL Forced</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="(host, i) in parseResult.proxy_hosts" :key="i" class="hover:bg-slate-50">
              <td class="px-5 py-2">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="d in host.domain_names"
                    :key="d"
                    class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700"
                    >{{ d }}</span
                  >
                </div>
              </td>
              <td class="px-5 py-2 font-mono text-xs text-slate-600">
                {{ host.forward_scheme }}://{{ host.forward_host }}:{{ host.forward_port }}
              </td>
              <td class="px-5 py-2">
                <span v-if="host.ssl_forced" class="text-xs font-medium text-green-600">Yes</span>
                <span v-else class="text-xs text-slate-400">No</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Redirection hosts table -->
      <div class="rounded-xl border border-slate-200 bg-white shadow-sm">
        <div class="border-b border-slate-200 px-5 py-3">
          <h3 class="text-sm font-semibold text-slate-800">
            Redirection Hosts
            <span class="ml-1 text-xs font-normal text-slate-400"
              >({{ parseResult.redirection_hosts.length }})</span
            >
          </h3>
        </div>
        <div
          v-if="parseResult.redirection_hosts.length === 0"
          class="px-5 py-6 text-center text-sm text-slate-400"
        >
          No redirection hosts found.
        </div>
        <table v-else class="w-full text-left text-sm">
          <thead class="border-b border-slate-100 bg-slate-50 text-xs font-medium text-slate-500">
            <tr>
              <th class="px-5 py-2">Domains</th>
              <th class="px-5 py-2">Forward To</th>
              <th class="px-5 py-2">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="(rh, i) in parseResult.redirection_hosts" :key="i" class="hover:bg-slate-50">
              <td class="px-5 py-2">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="d in rh.domain_names"
                    :key="d"
                    class="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700"
                    >{{ d }}</span
                  >
                </div>
              </td>
              <td class="px-5 py-2 font-mono text-xs text-slate-600">
                {{ rh.forward_scheme }}://{{ rh.forward_domain_name }}{{ rh.forward_path }}
              </td>
              <td class="px-5 py-2 text-xs text-slate-600">{{ rh.status_code }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Confirm error -->
      <div v-if="confirmError" class="rounded-lg bg-red-50 p-4 text-sm text-red-700">
        {{ confirmError }}
      </div>

      <!-- Action buttons -->
      <div class="flex items-center justify-end gap-3">
        <button
          class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          :disabled="confirming"
          @click="reset"
        >
          Start Over
        </button>
        <button
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
          :disabled="confirming"
          @click="doConfirm"
        >
          {{ confirming ? 'Importing…' : 'Confirm Import' }}
        </button>
      </div>
    </div>

    <!-- ── Step 3: Done ────────────────────────────────────────────────────── -->
    <div
      v-if="step === 'done' && importSummary"
      class="rounded-xl border border-slate-200 bg-white p-8 shadow-sm text-center"
    >
      <svg
        class="mx-auto mb-4 h-12 w-12 text-emerald-500"
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
      <h3 class="mb-2 text-base font-semibold text-slate-800">Import Complete</h3>
      <p class="text-sm text-slate-600">
        {{ importSummary.proxy_hosts_created }} proxy host(s) and
        {{ importSummary.redirection_hosts_created }} redirection host(s) were created.
      </p>

      <!-- Errors during import -->
      <div v-if="importSummary.errors.length" class="mt-4 rounded-lg bg-amber-50 p-4 text-left">
        <p class="mb-1 text-xs font-semibold text-amber-700">Some items could not be imported:</p>
        <ul class="list-inside list-disc space-y-1 text-xs text-amber-700">
          <li v-for="(err, i) in importSummary.errors" :key="i">{{ err }}</li>
        </ul>
      </div>

      <div class="mt-6 flex items-center justify-center gap-3">
        <button
          class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          @click="reset"
        >
          Import Another
        </button>
        <router-link
          to="/proxy-hosts"
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500"
        >
          View Proxy Hosts
        </router-link>
      </div>
    </div>
  </div>
</template>
