<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form, addHeaderRule, removeHeaderRule } = useProxyHostForm()

const presets: Array<{
  label: string
  direction: 'request' | 'response'
  operation: 'set' | 'add' | 'delete'
  name: string
  value: string
}> = [
  {
    label: 'Hide Server header',
    direction: 'response',
    operation: 'delete',
    name: 'Server',
    value: '',
  },
  {
    label: 'Hide X-Powered-By',
    direction: 'response',
    operation: 'delete',
    name: 'X-Powered-By',
    value: '',
  },
  {
    label: 'CORS Allow-Origin *',
    direction: 'response',
    operation: 'set',
    name: 'Access-Control-Allow-Origin',
    value: '*',
  },
  {
    label: 'X-Frame-Options DENY',
    direction: 'response',
    operation: 'set',
    name: 'X-Frame-Options',
    value: 'DENY',
  },
  {
    label: 'Cache-Control no-store',
    direction: 'response',
    operation: 'set',
    name: 'Cache-Control',
    value: 'no-store',
  },
  {
    label: 'X-Request-ID passthrough',
    direction: 'request',
    operation: 'set',
    name: 'X-Request-ID',
    value: '{http.request.uuid}',
  },
]
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p class="text-sm font-medium text-slate-700">Custom Header Injection</p>
      <p class="mt-1 text-xs text-slate-500">
        Add, set, or remove HTTP headers on requests sent to the upstream or responses returned to
        the client. Rules are applied in order. Request header changes are visible to the upstream;
        response header changes are visible to the browser.
      </p>
    </div>

    <!-- Rule rows -->
    <div
      v-if="form.custom_headers.length === 0"
      class="rounded-lg border border-dashed border-slate-300 py-8 text-center text-sm text-slate-400"
    >
      No header rules configured. Click "Add Rule" below.
    </div>

    <div
      v-for="(rule, i) in form.custom_headers"
      :key="i"
      class="grid grid-cols-12 items-end gap-2 rounded-lg border border-slate-200 p-3"
    >
      <!-- Direction -->
      <div class="col-span-2">
        <label class="mb-1 block text-xs font-medium text-slate-600">Direction</label>
        <select
          v-model="rule.direction"
          class="w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        >
          <option value="request">Request</option>
          <option value="response">Response</option>
        </select>
      </div>

      <!-- Operation -->
      <div class="col-span-2">
        <label class="mb-1 block text-xs font-medium text-slate-600">Operation</label>
        <select
          v-model="rule.operation"
          class="w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        >
          <option value="set">Set</option>
          <option value="add">Add</option>
          <option value="delete">Delete</option>
        </select>
      </div>

      <!-- Header name -->
      <div :class="rule.operation === 'delete' ? 'col-span-7' : 'col-span-4'">
        <label class="mb-1 block text-xs font-medium text-slate-600">Header Name</label>
        <input
          v-model="rule.name"
          type="text"
          placeholder="X-Custom-Header"
          class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
      </div>

      <!-- Value (hidden for delete) -->
      <div v-if="rule.operation !== 'delete'" class="col-span-3">
        <label class="mb-1 block text-xs font-medium text-slate-600">Value</label>
        <input
          v-model="rule.value"
          type="text"
          placeholder="header-value"
          class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
      </div>

      <!-- Remove button -->
      <div class="col-span-1 flex justify-end">
        <button
          class="rounded-lg p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-500"
          title="Remove rule"
          @click="removeHeaderRule(i)"
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
      class="flex items-center gap-1.5 text-sm text-indigo-600 hover:text-indigo-800"
      @click="addHeaderRule"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
      Add Rule
    </button>

    <!-- Quick-add common rules -->
    <div class="rounded-lg border border-slate-200 p-4">
      <p class="mb-3 text-xs font-medium text-slate-600">Common presets</p>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="preset in presets"
          :key="preset.label"
          class="rounded-full border border-slate-200 px-2.5 py-1 text-xs text-slate-600 hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700"
          @click="
            form.custom_headers.push({
              direction: preset.direction,
              operation: preset.operation,
              name: preset.name,
              value: preset.value,
            })
          "
        >
          + {{ preset.label }}
        </button>
      </div>
    </div>
  </div>
</template>
