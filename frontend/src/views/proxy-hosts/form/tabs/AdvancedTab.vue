<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form } = useProxyHostForm()
</script>

<template>
  <div class="space-y-4">
    <div>
      <label class="mb-1 block text-sm font-medium text-slate-700"> Advanced Configuration </label>
      <p class="mb-2 text-xs text-slate-400">
        Custom Caddy JSON snippet merged into this route's handler. Must be valid JSON.
      </p>
      <textarea
        v-model="form.advanced_config"
        rows="10"
        placeholder="{}"
        spellcheck="false"
        class="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
      />
    </div>
  </div>
</template>
