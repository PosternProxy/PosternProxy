<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form } = useProxyHostForm()
</script>

<template>
  <div class="space-y-5">
    <!-- Master toggle -->
    <label class="flex cursor-pointer items-center gap-3 rounded-lg border border-slate-200 p-4">
      <input
        v-model="form.hc_enabled"
        type="checkbox"
        class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
      />
      <div>
        <p class="text-sm font-medium text-slate-800">Enable Health Checks</p>
        <p class="text-xs text-slate-500">
          Caddy will monitor this upstream and remove it from load balancing when it becomes
          unhealthy.
        </p>
      </div>
    </label>

    <template v-if="form.hc_enabled">
      <!-- Active health checks -->
      <div class="rounded-lg border border-slate-200 p-4">
        <h4 class="mb-3 text-sm font-semibold text-slate-700">Active Health Checks</h4>
        <p class="mb-4 text-xs text-slate-500">
          Caddy proactively polls the upstream on a set interval.
        </p>

        <div class="space-y-3">
          <!-- Health check path -->
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600">Health Check Path</label>
            <input
              v-model="form.hc_active_path"
              type="text"
              placeholder="/health"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">Leave empty to disable active checks</p>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <!-- Interval -->
            <div>
              <label class="mb-1 block text-xs font-medium text-slate-600">Interval (s)</label>
              <input
                v-model.number="form.hc_active_interval"
                type="number"
                min="5"
                max="3600"
                placeholder="30"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <!-- Timeout -->
            <div>
              <label class="mb-1 block text-xs font-medium text-slate-600">Timeout (s)</label>
              <input
                v-model.number="form.hc_active_timeout"
                type="number"
                min="1"
                max="60"
                placeholder="5"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>

            <!-- Expected status -->
            <div>
              <label class="mb-1 block text-xs font-medium text-slate-600">Expect Status</label>
              <input
                v-model.number="form.hc_active_expect_status"
                type="number"
                min="100"
                max="599"
                placeholder="200"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- Passive health checks -->
      <div class="rounded-lg border border-slate-200 p-4">
        <h4 class="mb-3 text-sm font-semibold text-slate-700">Passive Health Checks</h4>
        <p class="mb-4 text-xs text-slate-500">
          Caddy marks upstreams unhealthy based on errors observed in live traffic. Set to 0 to
          disable individual options.
        </p>

        <div class="grid grid-cols-3 gap-3">
          <!-- Fail duration -->
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600">Fail Window (s)</label>
            <input
              v-model.number="form.hc_passive_fail_duration"
              type="number"
              min="0"
              max="3600"
              placeholder="30"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">How long to mark as unhealthy</p>
          </div>

          <!-- Max fails -->
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600">Max Fails</label>
            <input
              v-model.number="form.hc_passive_max_fails"
              type="number"
              min="0"
              max="100"
              placeholder="3"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">Errors before marked down</p>
          </div>

          <!-- Unhealthy latency -->
          <div>
            <label class="mb-1 block text-xs font-medium text-slate-600"
              >Latency Threshold (ms)</label
            >
            <input
              v-model.number="form.hc_passive_unhealthy_latency"
              type="number"
              min="0"
              max="30000"
              placeholder="0 (disabled)"
              class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <p class="mt-1 text-xs text-slate-400">0 = disabled</p>
          </div>
        </div>
      </div>
    </template>

    <!-- When disabled: informational note -->
    <template v-else>
      <div class="rounded-lg border border-dashed border-slate-300 p-6 text-center">
        <svg
          class="mx-auto mb-2 h-8 w-8 text-slate-300"
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
        <p class="text-sm text-slate-500">Health checks are disabled for this host.</p>
        <p class="mt-1 text-xs text-slate-400">
          Enable health checks above to monitor upstream availability and show live status in the
          hosts table.
        </p>
      </div>
    </template>
  </div>
</template>
