<script setup lang="ts">
// Shell + tab-nav + basic-mode for the proxy-host create/edit form.
//
// Per-tab content lives in form/tabs/. Form state, helpers, validation, and
// the create/update API calls live in form/useProxyHostForm.ts as a shared
// composable (provide/inject) — every tab subscribes to the same form via
// useProxyHostForm() so there is no prop-drilling of state.

import { watch } from 'vue'
import type { ProxyHost, AccessList, Certificate } from '@/types'
import { provideProxyHostForm } from './form/useProxyHostForm'
import DetailsTab from './form/tabs/DetailsTab.vue'
import UpstreamsTab from './form/tabs/UpstreamsTab.vue'
import LocationsTab from './form/tabs/LocationsTab.vue'
import SslTab from './form/tabs/SslTab.vue'
import HealthTab from './form/tabs/HealthTab.vue'
import SecurityTab from './form/tabs/SecurityTab.vue'
import ErrorsTab from './form/tabs/ErrorsTab.vue'
import HeadersTab from './form/tabs/HeadersTab.vue'
import AdvancedTab from './form/tabs/AdvancedTab.vue'

const props = defineProps<{
  modelValue: boolean
  host: ProxyHost | null
  defaultServerId: number
  accessLists: AccessList[]
  certificates: Certificate[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: [host: ProxyHost]
}>()

// One call: builds the form API and provides it to descendant tabs.
const f = provideProxyHostForm()

// Re-init the form whenever the modal opens or the edited host changes.
watch(
  () => [props.modelValue, props.host] as const,
  ([open]) => {
    if (open) f.init(props.host, props.defaultServerId)
  }
)

function close() {
  emit('update:modelValue', false)
}

async function onSave() {
  try {
    const saved = await f.save(props.host)
    emit('saved', saved)
    close()
  } catch {
    // saveError is already populated by the composable; nothing else to do.
  }
}

function showAdvancedMode() {
  f.showAdvanced.value = true
  f.activeTab.value = 'details'
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="close"
    >
      <div class="flex w-full max-w-2xl flex-col rounded-2xl bg-white shadow-2xl">
        <!-- Modal header -->
        <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h3 class="text-base font-semibold text-slate-800">
            {{ host === null ? 'Add Proxy Host' : 'Edit Proxy Host' }}
          </h3>
          <button
            class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            @click="close"
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

        <!-- Tabs (advanced mode only) -->
        <div v-if="f.showAdvanced.value" class="flex border-b border-slate-200 px-6">
          <button
            v-for="tab in [
              'details',
              'upstreams',
              'locations',
              'ssl',
              'health',
              'security',
              'errors',
              'headers',
              'advanced',
            ] as const"
            :key="tab"
            class="mr-6 border-b-2 py-3 text-sm font-medium capitalize transition-colors"
            :class="
              f.activeTab.value === tab
                ? 'border-indigo-600 text-indigo-600'
                : 'border-transparent text-slate-500 hover:text-slate-700'
            "
            @click="f.activeTab.value = tab"
          >
            {{ tab }}
          </button>
        </div>

        <!-- Modal body -->
        <div class="flex-1 overflow-y-auto px-6 py-5" style="max-height: 60vh">
          <!-- Error -->
          <div v-if="f.saveError.value" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
            {{ f.saveError.value }}
          </div>

          <!-- ── Basic mode: single-page essentials ── -->
          <template v-if="!f.showAdvanced.value">
            <div class="space-y-4">
              <!-- Domain names -->
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">Domain Names</label>
                <div
                  class="flex flex-wrap gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500"
                >
                  <span
                    v-for="(d, i) in f.form.domain_names"
                    :key="d"
                    class="flex items-center gap-1 rounded bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700"
                  >
                    {{ d }}
                    <button type="button" class="hover:text-indigo-900" @click="f.removeDomain(i)">
                      &times;
                    </button>
                  </span>
                  <input
                    v-model="f.form.domainInput"
                    type="text"
                    placeholder="example.com"
                    class="min-w-32 flex-1 border-0 bg-transparent p-0 text-sm focus:outline-none focus:ring-0"
                    @keydown="f.domainKeydown"
                    @blur="f.addDomain"
                  />
                </div>
                <p class="mt-1 text-xs text-slate-400">Press Enter or comma to add each domain</p>
              </div>

              <!-- Scheme + Host + Port -->
              <div class="grid grid-cols-12 gap-3">
                <div class="col-span-3">
                  <label class="mb-1 block text-sm font-medium text-slate-700">Scheme</label>
                  <select
                    v-model="f.form.forward_scheme"
                    class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <option value="http">http</option>
                    <option value="https">https</option>
                  </select>
                </div>
                <div class="col-span-6">
                  <label class="mb-1 block text-sm font-medium text-slate-700"
                    >Forward Hostname / IP</label
                  >
                  <input
                    v-model="f.form.forward_host"
                    type="text"
                    placeholder="192.168.1.10"
                    class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                </div>
                <div class="col-span-3">
                  <label class="mb-1 block text-sm font-medium text-slate-700">Port</label>
                  <input
                    v-model.number="f.form.forward_port"
                    type="number"
                    min="1"
                    max="65535"
                    placeholder="80"
                    class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                </div>
              </div>

              <!-- SSL mode -->
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">SSL / TLS</label>
                <div class="flex gap-2">
                  <button
                    v-for="opt in [
                      { value: 'auto', label: 'Auto HTTPS', desc: 'Let\'s Encrypt via Caddy' },
                      { value: 'https', label: 'Custom Cert', desc: 'Use a stored certificate' },
                      { value: 'http', label: 'HTTP Only', desc: 'No TLS (internal hosts)' },
                    ] as const"
                    :key="opt.value"
                    type="button"
                    class="flex-1 rounded-lg border px-3 py-2 text-left text-xs transition-colors"
                    :class="
                      f.sslMode.value === opt.value
                        ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
                        : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'
                    "
                    @click="f.setSslMode(opt.value)"
                  >
                    <div class="font-medium">{{ opt.label }}</div>
                    <div class="mt-0.5 opacity-70">{{ opt.desc }}</div>
                  </button>
                </div>
                <div v-if="f.sslMode.value === 'https'" class="mt-2">
                  <select
                    v-model="f.form.certificate_id"
                    class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  >
                    <option :value="null">— select certificate —</option>
                    <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
                      {{ cert.name }}
                    </option>
                  </select>
                </div>
                <p v-if="f.sslMode.value === 'http'" class="mt-1 text-xs text-amber-600">
                  Host will be served over plain HTTP on port 80. Use for internal services that
                  can't satisfy Let's Encrypt HTTP-01 challenges.
                </p>
              </div>

              <!-- Access list -->
              <div>
                <label class="mb-1 block text-sm font-medium text-slate-700">Access List</label>
                <select
                  v-model="f.form.access_list_id"
                  class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                >
                  <option :value="null">None</option>
                  <option v-for="al in accessLists" :key="al.id" :value="al.id">
                    {{ al.name }}
                  </option>
                </select>
              </div>

              <!-- Feature toggles -->
              <div class="grid grid-cols-2 gap-2 rounded-lg border border-slate-200 p-4">
                <label class="flex cursor-pointer items-center gap-2 text-sm text-slate-700">
                  <input
                    v-model="f.form.websocket_support"
                    type="checkbox"
                    class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                  />
                  WebSocket Support
                </label>
              </div>

              <!-- Advanced toggle -->
              <button
                type="button"
                class="flex items-center gap-1.5 text-sm text-slate-500 hover:text-indigo-600"
                @click="showAdvancedMode"
              >
                <svg
                  class="h-4 w-4 transition-transform"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
                  />
                </svg>
                Advanced Settings
              </button>
            </div>
          </template>

          <!-- ── Advanced mode: dispatch to per-tab component ── -->
          <template v-else>
            <DetailsTab v-if="f.activeTab.value === 'details'" />
            <UpstreamsTab v-else-if="f.activeTab.value === 'upstreams'" />
            <LocationsTab v-else-if="f.activeTab.value === 'locations'" />
            <SslTab
              v-else-if="f.activeTab.value === 'ssl'"
              :access-lists="accessLists"
              :certificates="certificates"
            />
            <HealthTab v-else-if="f.activeTab.value === 'health'" />
            <SecurityTab v-else-if="f.activeTab.value === 'security'" />
            <ErrorsTab v-else-if="f.activeTab.value === 'errors'" />
            <HeadersTab v-else-if="f.activeTab.value === 'headers'" />
            <AdvancedTab v-else-if="f.activeTab.value === 'advanced'" />
          </template>
        </div>

        <!-- Modal footer -->
        <div class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4">
          <button
            class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            :disabled="f.saving.value"
            @click="close"
          >
            Cancel
          </button>
          <button
            class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
            :disabled="f.saving.value"
            @click="onSave"
          >
            {{ f.saving.value ? 'Saving…' : host === null ? 'Create' : 'Save Changes' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
