<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form, addDomain, removeDomain, domainKeydown } = useProxyHostForm()
</script>

<template>
  <div class="space-y-4">
    <!-- Domain names -->
    <div>
      <label class="mb-1 block text-sm font-medium text-slate-700">Domain Names</label>
      <div
        class="flex flex-wrap gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-within:border-indigo-500 focus-within:ring-1 focus-within:ring-indigo-500"
      >
        <span
          v-for="(d, i) in form.domain_names"
          :key="d"
          class="flex items-center gap-1 rounded bg-indigo-100 px-2 py-0.5 text-xs font-medium text-indigo-700"
        >
          {{ d }}
          <button type="button" class="hover:text-indigo-900" @click="removeDomain(i)">
            &times;
          </button>
        </span>
        <input
          v-model="form.domainInput"
          type="text"
          placeholder="example.com"
          class="min-w-32 flex-1 border-0 bg-transparent p-0 text-sm focus:outline-none focus:ring-0"
          @keydown="domainKeydown"
          @blur="addDomain"
        />
      </div>
      <p class="mt-1 text-xs text-slate-400">Press Enter or comma to add each domain</p>
    </div>

    <!-- Scheme + Host + Port -->
    <div class="grid grid-cols-12 gap-3">
      <div class="col-span-3">
        <label class="mb-1 block text-sm font-medium text-slate-700">Scheme</label>
        <select
          v-model="form.forward_scheme"
          class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        >
          <option value="http">http</option>
          <option value="https">https</option>
        </select>
      </div>
      <div class="col-span-6">
        <label class="mb-1 block text-sm font-medium text-slate-700">Forward Hostname / IP</label>
        <input
          v-model="form.forward_host"
          type="text"
          placeholder="192.168.1.10"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
      </div>
      <div class="col-span-3">
        <label class="mb-1 block text-sm font-medium text-slate-700">Port</label>
        <input
          v-model.number="form.forward_port"
          type="number"
          min="1"
          max="65535"
          placeholder="80"
          class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
      </div>
    </div>

    <!-- Feature toggles -->
    <div class="grid grid-cols-2 gap-2 rounded-lg border border-slate-200 p-4">
      <label class="flex cursor-pointer items-center gap-2 text-sm text-slate-700">
        <input
          v-model="form.websocket_support"
          type="checkbox"
          class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
        />
        WebSocket Support
      </label>
    </div>
  </div>
</template>
