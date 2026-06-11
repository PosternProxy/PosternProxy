<script setup lang="ts">
import { useProxyHostForm } from '../useProxyHostForm'

const { form, addExtraUpstream, removeExtraUpstream } = useProxyHostForm()
</script>

<template>
  <div class="space-y-4">
    <!-- Primary upstream (read-only summary) -->
    <div class="rounded-lg border border-slate-200 bg-slate-50 p-4">
      <p class="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
        Primary Upstream
      </p>
      <p class="font-mono text-sm text-slate-700">
        {{ form.forward_scheme }}://{{ form.forward_host || '—' }}:{{ form.forward_port }}
      </p>
      <p class="mt-1 text-xs text-slate-400">
        Configured on the Details tab. Always included first in the upstream pool.
      </p>
    </div>

    <!-- Extra upstream rows -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <p class="text-sm font-medium text-slate-700">Additional Backends</p>
        <span
          class="rounded-full px-2 py-0.5 text-xs font-medium"
          :class="
            form.extra_upstreams.length > 0
              ? 'bg-indigo-100 text-indigo-700'
              : 'bg-slate-100 text-slate-500'
          "
        >
          {{ form.extra_upstreams.length + 1 }} upstream{{
            form.extra_upstreams.length + 1 === 1 ? '' : 's'
          }}
          total
        </span>
      </div>

      <div
        v-if="form.extra_upstreams.length === 0"
        class="rounded-lg border border-dashed border-slate-300 py-6 text-center text-sm text-slate-400"
      >
        No extra backends. Add one below to enable load balancing.
      </div>

      <div
        v-for="(up, i) in form.extra_upstreams"
        :key="i"
        class="grid grid-cols-12 items-end gap-2 rounded-lg border border-slate-200 p-3"
      >
        <div class="col-span-1 flex items-center justify-center pb-1.5">
          <span class="text-xs font-semibold text-slate-400">#{{ i + 2 }}</span>
        </div>
        <div class="col-span-7">
          <label class="mb-1 block text-xs font-medium text-slate-600">Host / IP</label>
          <input
            v-model="up.host"
            type="text"
            placeholder="192.168.1.11"
            class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
        <div class="col-span-3">
          <label class="mb-1 block text-xs font-medium text-slate-600">Port</label>
          <input
            v-model.number="up.port"
            type="number"
            min="1"
            max="65535"
            placeholder="80"
            class="w-full rounded-lg border border-slate-300 px-2 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
        <div class="col-span-1 flex justify-end">
          <button
            class="rounded-lg p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-500"
            title="Remove upstream"
            @click="removeExtraUpstream(i)"
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
        @click="addExtraUpstream"
      >
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 4v16m8-8H4"
          />
        </svg>
        Add Backend
      </button>
    </div>

    <!-- LB policy — only shown when there are extra upstreams -->
    <Transition name="fade">
      <div
        v-if="form.extra_upstreams.length > 0"
        class="rounded-lg border border-indigo-200 bg-indigo-50 p-4"
      >
        <p class="mb-3 text-sm font-semibold text-indigo-800">Load Balancing Policy</p>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <label
            v-for="policy in [
              {
                value: 'round_robin',
                label: 'Round Robin',
                desc: 'Cycles through backends in order',
              },
              { value: 'random', label: 'Random', desc: 'Picks a random backend each request' },
              {
                value: 'least_conn',
                label: 'Least Conn',
                desc: 'Routes to the least-busy backend',
              },
              { value: 'ip_hash', label: 'IP Hash', desc: 'Sticky sessions by client IP' },
              { value: 'uri_hash', label: 'URI Hash', desc: 'Sticky sessions by request path' },
              { value: 'first', label: 'First', desc: 'Always uses first healthy backend' },
            ]"
            :key="policy.value"
            class="flex cursor-pointer items-start gap-2 rounded-lg border p-3 transition-colors"
            :class="
              form.lb_policy === policy.value
                ? 'border-indigo-400 bg-white shadow-sm'
                : 'border-transparent bg-white/60 hover:border-indigo-200 hover:bg-white'
            "
          >
            <input
              v-model="form.lb_policy"
              type="radio"
              :value="policy.value"
              class="mt-0.5 h-3.5 w-3.5 border-slate-300 text-indigo-600 focus:ring-indigo-500"
            />
            <div>
              <p class="text-xs font-semibold text-slate-800">{{ policy.label }}</p>
              <p class="mt-0.5 text-xs text-slate-500">{{ policy.desc }}</p>
            </div>
          </label>
        </div>
        <p class="mt-3 text-xs text-indigo-700">
          Caddy automatically retries failed requests on other backends. Pair with
          <strong>Health Checks</strong> to remove unhealthy backends from rotation.
        </p>
      </div>
    </Transition>
  </div>
</template>
