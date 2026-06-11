<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form } = useProxyHostForm()
</script>

<template>
  <div class="space-y-5">
    <!-- TLS Passthrough -->
    <div class="rounded-lg border border-slate-200 p-4">
      <label class="flex cursor-pointer items-start gap-3">
        <input
          v-model="form.tls_passthrough"
          type="checkbox"
          class="mt-0.5 h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
        />
        <div>
          <p class="text-sm font-semibold text-slate-800">TLS Passthrough</p>
          <p class="mt-0.5 text-xs text-slate-500">
            Caddy will tunnel raw TLS traffic directly to the upstream without terminating SSL. The
            upstream is responsible for TLS. Certificate and HSTS settings are ignored when this is
            enabled.
          </p>
        </div>
      </label>
      <div
        v-if="form.tls_passthrough"
        class="mt-3 rounded-lg bg-purple-50 p-3 text-xs text-purple-700"
      >
        TLS Passthrough is active. Caddy will not inspect or modify TLS traffic for this host. SSL
        certificate selection on the SSL tab has no effect.
      </div>
    </div>

    <!-- Rate Limiting -->
    <div class="rounded-lg border border-slate-200 p-4">
      <label class="mb-3 flex cursor-pointer items-start gap-3">
        <input
          v-model="form.rate_limit_enabled"
          type="checkbox"
          class="mt-0.5 h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
        />
        <div>
          <p class="text-sm font-semibold text-slate-800">Enable Rate Limiting</p>
          <p class="mt-0.5 text-xs text-slate-500">
            Limit the number of requests per client IP within a sliding time window.
          </p>
        </div>
      </label>

      <template v-if="form.rate_limit_enabled">
        <div class="mt-4 grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600"> Max Requests </label>
            <input
              v-model.number="form.rate_limit_requests"
              type="number"
              min="1"
              max="100000"
              placeholder="100"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">Number of requests allowed per window</p>
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600"> Time Window </label>
            <input
              v-model="form.rate_limit_window"
              type="text"
              placeholder="1m"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">
              Examples: <code class="font-mono">30s</code>, <code class="font-mono">1m</code>,
              <code class="font-mono">1h</code>
            </p>
          </div>
        </div>

        <div class="mt-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-700">
          Clients exceeding the limit will receive a <strong>429 Too Many Requests</strong> response
          until their window resets.
        </div>
      </template>

      <div v-else class="mt-3 rounded-lg border border-dashed border-slate-200 p-4 text-center">
        <p class="text-xs text-slate-400">
          Rate limiting is disabled. Enable it above to restrict request frequency.
        </p>
      </div>
    </div>
  </div>
</template>
