<script setup lang="ts">
import type { AccessList, Certificate } from '@/types'
import { useProxyHostForm } from '../useProxyHostForm'

defineProps<{
  accessLists: AccessList[]
  certificates: Certificate[]
}>()

const { form, sslMode, setSslMode } = useProxyHostForm()
</script>

<template>
  <div class="space-y-4">
    <!-- TLS passthrough warning -->
    <div v-if="form.tls_passthrough" class="rounded-lg bg-purple-50 p-3 text-xs text-purple-700">
      TLS Passthrough is enabled on the Security tab. Certificate and HSTS settings below are
      ignored when passthrough is active — Caddy will tunnel raw TLS to the upstream.
    </div>

    <!-- SSL mode buttons -->
    <div>
      <label class="mb-1 block text-sm font-medium text-slate-700">SSL Mode</label>
      <div class="flex gap-2">
        <button
          v-for="opt in [
            { value: 'auto', label: 'Auto HTTPS', desc: 'Let\'s Encrypt via Caddy' },
            { value: 'https', label: 'Custom Cert', desc: 'Use a stored certificate' },
            { value: 'http', label: 'HTTP Only', desc: 'No TLS (internal hosts)' },
          ] as const"
          :key="opt.value"
          type="button"
          :disabled="form.tls_passthrough"
          class="flex-1 rounded-lg border px-3 py-2 text-left text-xs transition-colors disabled:opacity-40"
          :class="
            sslMode === opt.value
              ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
              : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'
          "
          @click="setSslMode(opt.value)"
        >
          <div class="font-medium">{{ opt.label }}</div>
          <div class="mt-0.5 opacity-70">{{ opt.desc }}</div>
        </button>
      </div>
      <p v-if="sslMode === 'http'" class="mt-1 text-xs text-amber-600">
        Host served over plain HTTP on port 80. TLS is disabled — no certificate, no HSTS.
      </p>
    </div>

    <!-- Certificate selector (only when custom cert mode) -->
    <div v-if="sslMode === 'https'">
      <label class="mb-1 block text-sm font-medium text-slate-700">Certificate</label>
      <select
        v-model="form.certificate_id"
        :disabled="form.tls_passthrough"
        class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:opacity-50"
      >
        <option :value="null">— select certificate —</option>
        <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
          {{ cert.name }}
        </option>
      </select>
    </div>
    <div v-if="sslMode === 'auto'" class="text-xs text-slate-400">
      Caddy will automatically obtain and renew a Let's Encrypt certificate for the configured
      domain(s).
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium text-slate-700">Access List</label>
      <select
        v-model="form.access_list_id"
        class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
      >
        <option :value="null">None</option>
        <option v-for="al in accessLists" :key="al.id" :value="al.id">{{ al.name }}</option>
      </select>
    </div>

    <!-- HSTS toggles — hidden for HTTP-only hosts -->
    <div
      v-if="sslMode !== 'http'"
      class="grid grid-cols-2 gap-2 rounded-lg border border-slate-200 p-4"
      :class="{ 'opacity-50': form.tls_passthrough }"
    >
      <label class="flex cursor-pointer items-center gap-2 text-sm text-slate-700">
        <input
          v-model="form.hsts_enabled"
          type="checkbox"
          :disabled="form.tls_passthrough"
          class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
        />
        HSTS Enabled
      </label>
      <label class="flex cursor-pointer items-center gap-2 text-sm text-slate-700">
        <input
          v-model="form.hsts_subdomains"
          type="checkbox"
          :disabled="form.tls_passthrough"
          class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
        />
        HSTS Subdomains
      </label>
    </div>
  </div>
</template>
