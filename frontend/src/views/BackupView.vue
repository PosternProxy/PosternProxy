<script setup lang="ts">
import { ref } from 'vue'
import { exportBackup, importBackup } from '@/api/backup'

const exportPassphrase = ref('')
const exportConfirmation = ref('')
const restorePassphrase = ref('')
const createUnenrolled = ref(false)
const mappingJSON = ref('{}')
const exportError = ref('')
const exporting = ref(false)
const importFile = ref<File | null>(null)
const importing = ref(false)
const importError = ref('')
const importSuccess = ref('')
const showConfirm = ref(false)

function handleFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  importFile.value = target.files?.[0] ?? null
  importError.value = ''
  importSuccess.value = ''
}

async function triggerExport() {
 exportError.value = ''
 if(exportPassphrase.value.length < 12 || exportPassphrase.value !== exportConfirmation.value){exportError.value='Use a passphrase of at least 12 characters and matching confirmation.';return}
 exporting.value=true
 try{await exportBackup(exportPassphrase.value);exportPassphrase.value='';exportConfirmation.value=''}
 catch(e: unknown){exportError.value=e instanceof Error?e.message:'Export failed'}
 finally{exporting.value=false}
}

function openConfirm() {
  if (!importFile.value) {
    importError.value = 'Please select a backup file to import.'
    return
  }
  importError.value = ''
  importSuccess.value = ''
  showConfirm.value = true
}

async function doImport() {
  if (!importFile.value) return
  importing.value = true
  importError.value = ''
  importSuccess.value = ''
  showConfirm.value = false
  try {
    const result = await importBackup(importFile.value, restorePassphrase.value, createUnenrolled.value, JSON.parse(mappingJSON.value) as Record<string, number>)
    importSuccess.value = result.synced
      ? `${result.restored} record(s) restored and configuration applied.`
      : `${result.restored} record(s) restored. Configuration is still waiting to apply; check server configuration status.`
    restorePassphrase.value = ''
    importFile.value = null
    // Reset file input
    const input = document.getElementById('backup-file-input') as HTMLInputElement | null
    if (input) input.value = ''
  } catch (e: unknown) {
    importError.value =
      e instanceof Error ? e.message : 'Import failed. Please check the file and try again.'
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-8">
    <!-- Page header -->
    <div>
      <h2 class="text-xl font-semibold text-slate-800">Backup &amp; Restore</h2>
      <p class="mt-0.5 text-sm text-slate-500">
        Export an encrypted recovery archive or restore configuration. Users, sessions and enrollment credentials remain on the destination.
      </p>
    </div>

    <!-- Export section -->
    <div class="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <div class="mb-4 flex items-start gap-4">
        <div
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
        </div>
        <div>
          <h3 class="text-sm font-semibold text-slate-800">Export Configuration</h3>
          <p class="mt-1 text-sm text-slate-500">
            Download a passphrase-encrypted archive containing hosts, access lists, uploaded certificate bytes and server descriptions. Keep the passphrase separately; it cannot be recovered.
          </p>
        </div>
      </div>
      <p v-if="exportError" class="text-sm text-red-700">{{ exportError }}</p>
      <div class="mb-4 space-y-2">
        <input v-model="exportPassphrase" type="password" autocomplete="new-password" placeholder="Backup passphrase (12+ characters)" aria-label="Backup passphrase" class="w-full rounded border p-2" />
        <input v-model="exportConfirmation" type="password" autocomplete="new-password" placeholder="Confirm passphrase" aria-label="Confirm backup passphrase" class="w-full rounded border p-2" />
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="triggerExport" :disabled="exporting"
      >
        Export Backup
      </button>
    </div>

    <!-- Import section -->
    <div class="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <div class="mb-4 flex items-start gap-4">
        <div
          class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-50 text-amber-600"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l4-4m0 0l4 4m-4-4v12"
            />
          </svg>
        </div>
        <div>
          <h3 class="text-sm font-semibold text-slate-800">Restore from Backup</h3>
          <p class="mt-1 text-sm text-slate-500">
            Upload a previously exported JSON backup file. Existing records will be overwritten.
          </p>
        </div>
      </div>

      <!-- Error / Success banners -->
      <div v-if="importError" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">
        {{ importError }}
      </div>
      <div v-if="importSuccess" class="mb-4 rounded-lg bg-emerald-50 p-4 text-sm text-emerald-700">
        {{ importSuccess }}
      </div>

      <div class="mb-4 space-y-3">
        <input v-model="restorePassphrase" type="password" autocomplete="off" placeholder="Archive passphrase" aria-label="Archive passphrase" class="w-full rounded border p-2" />
        <label class="block text-sm">Server mapping (source ID to existing destination ID)
          <textarea v-model="mappingJSON" class="mt-1 w-full rounded border p-2 font-mono" placeholder='{"2":52}'></textarea>
        </label>
        <label class="block text-sm"><input v-model="createUnenrolled" type="checkbox" /> Create unmapped remote servers without enrollment credentials. Enroll each agent afterward.</label>
        <p class="text-xs text-slate-500">Legacy version 1 JSON restores only database configuration. It requires the original certificate files and matching server IDs.</p>
      </div>
      <!-- File picker -->
      <div class="mb-4">
        <label class="mb-1 block text-sm font-medium text-slate-700">Backup File (.json)</label>
        <input
          id="backup-file-input"
          type="file"
          accept=".json,application/json"
          class="block w-full cursor-pointer rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 file:mr-3 file:rounded file:border-0 file:bg-indigo-50 file:px-3 file:py-1 file:text-xs file:font-medium file:text-indigo-700 hover:file:bg-indigo-100 focus:outline-none"
          @change="handleFileChange"
        />
      </div>

      <div class="rounded-lg bg-amber-50 p-3 text-xs text-amber-700">
        Warning: importing a backup will overwrite the current configuration. This action cannot be
        undone.
      </div>

      <div class="mt-4">
        <button
          class="rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-500 disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2"
          :disabled="importing || !importFile"
          @click="openConfirm"
        >
          {{ importing ? 'Importing…' : 'Import Backup' }}
        </button>
      </div>
    </div>

    <!-- Confirm dialog -->
    <Teleport to="body">
      <div
        v-if="showConfirm"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="showConfirm = false"
      >
        <div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-2xl">
          <h3 class="mb-2 text-base font-semibold text-slate-800">Confirm Import</h3>
          <p class="text-sm text-slate-600">
            You are about to restore the configuration from
            <strong>{{ importFile?.name }}</strong
            >. All existing records will be overwritten. This cannot be undone.
          </p>
          <div class="mt-5 flex justify-end gap-3">
            <button
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
              @click="showConfirm = false"
            >
              Cancel
            </button>
            <button
              class="rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-500"
              @click="doImport"
            >
              Yes, Import
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
