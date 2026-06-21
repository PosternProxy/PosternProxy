<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form } = useProxyHostForm()
</script>

<template>
  <div class="space-y-5">
    <div class="rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p class="text-sm font-medium text-slate-700">Custom Error Pages</p>
      <p class="mt-1 text-xs text-slate-500">
        Enter custom HTML for specific HTTP error codes. When Caddy returns that error for this
        host, your page is served instead of the default host-down page. Leave blank to use the
        built-in PosternProxy response.
      </p>
    </div>

    <div
      v-for="(label, code) in { '502': 'Bad Gateway', '503': 'Service Unavailable' }"
      :key="code"
      class="rounded-lg border border-slate-200 p-4"
    >
      <div class="mb-2 flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="rounded-full bg-red-100 px-2 py-0.5 text-xs font-semibold text-red-700">{{
            code
          }}</span>
          <span class="text-sm font-medium text-slate-700">{{ label }}</span>
        </div>
        <button
          v-if="form.error_pages[code]"
          class="text-xs text-slate-400 hover:text-red-500"
          @click="form.error_pages[code] = ''"
        >
          Clear
        </button>
      </div>

      <textarea
        v-model="form.error_pages[code]"
        rows="6"
        spellcheck="false"
        :placeholder="
          code === '502'
            ? `<!-- Custom 502 page for this host -->\n<!DOCTYPE html>\n<html>\n<head><title>Bad Gateway</title></head>\n<body><h1>Service Temporarily Unavailable</h1></body>\n</html>`
            : `<!-- Custom 503 page for this host -->\n<!DOCTYPE html>\n<html>\n<head><title>Service Unavailable</title></head>\n<body><h1>Maintenance in Progress</h1></body>\n</html>`
        "
        class="w-full rounded-lg border border-slate-300 px-3 py-2 font-mono text-xs focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
      />

      <p class="mt-1 text-xs text-slate-400">
        Full HTML document. Caddy sets
        <code class="font-mono">Content-Type: text/html</code> automatically.
      </p>
    </div>

    <div class="rounded-lg bg-blue-50 p-3 text-xs text-blue-700">
      <strong>Note:</strong> These pages fire when the upstream is unreachable (502) or overloaded
      (503). They apply to both HTTP and HTTPS routes after the host is saved.
    </div>
  </div>
</template>
