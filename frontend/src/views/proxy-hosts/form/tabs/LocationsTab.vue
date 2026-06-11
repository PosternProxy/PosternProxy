<script setup lang="ts">
import type { ProxyHostLocation } from '@/types'
import { useProxyHostForm } from '../useProxyHostForm'

const { form, addLocation, removeLocation } = useProxyHostForm()
</script>

<template>
  <div class="space-y-3">
    <p class="text-xs text-slate-500">
      Custom locations route specific paths to different upstreams, overriding the default upstream
      for matching requests.
    </p>
    <div
      v-if="!(form.locations as ProxyHostLocation[])?.length"
      class="rounded-lg border border-dashed border-slate-300 py-6 text-center text-sm text-slate-400"
    >
      No custom locations. Click "Add Location" to add one.
    </div>
    <div
      v-for="(loc, i) in form.locations as ProxyHostLocation[]"
      :key="i"
      class="rounded-lg border border-slate-200 p-4"
    >
      <div class="mb-2 flex items-center justify-between">
        <span class="text-xs font-medium text-slate-600">Location {{ i + 1 }}</span>
        <button class="text-xs text-red-500 hover:text-red-700" @click="removeLocation(i)">
          Remove
        </button>
      </div>
      <div class="grid grid-cols-12 gap-2">
        <div class="col-span-4">
          <label class="mb-1 block text-xs font-medium text-slate-600">Path</label>
          <input
            v-model="loc.path"
            type="text"
            placeholder="/api"
            class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
        <div class="col-span-2">
          <label class="mb-1 block text-xs font-medium text-slate-600">Scheme</label>
          <select
            v-model="loc.forward_scheme"
            class="w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            <option value="http">http</option>
            <option value="https">https</option>
          </select>
        </div>
        <div class="col-span-4">
          <label class="mb-1 block text-xs font-medium text-slate-600">Host</label>
          <input
            v-model="loc.forward_host"
            type="text"
            placeholder="backend-host"
            class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
        <div class="col-span-2">
          <label class="mb-1 block text-xs font-medium text-slate-600">Port</label>
          <input
            v-model.number="loc.forward_port"
            type="number"
            min="1"
            max="65535"
            class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
      </div>
    </div>
    <button
      class="flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-800"
      @click="addLocation"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
      Add Location
    </button>
  </div>
</template>
