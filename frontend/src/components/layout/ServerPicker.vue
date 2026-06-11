<script setup lang="ts">
import { ref } from 'vue'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'

const serverStore = useServerStore()
const ctx = useServerContext()

const open = ref(false)

function statusColor(status: string | undefined | null): string {
  if (status === 'online') return 'bg-green-400'
  if (!status || status === 'offline') return 'bg-red-400'
  return 'bg-amber-400'
}

function close() {
  open.value = false
}

function pick(id: number) {
  ctx.select(id)
  close()
}
</script>

<template>
  <div v-if="serverStore.hasRemoteServers" class="px-2 pb-3">
    <p class="mb-1 px-1 text-[10px] font-semibold uppercase tracking-widest text-slate-500">
      Server
    </p>
    <div class="relative">
      <button
        class="flex w-full items-center gap-2 rounded-lg bg-slate-800 px-3 py-2 text-sm text-white hover:bg-slate-700 focus:outline-none"
        @click="open = !open"
      >
        <span
          class="h-2 w-2 shrink-0 rounded-full"
          :class="statusColor(ctx.selectedServer?.status)"
        />
        <span class="flex-1 truncate text-left">{{ ctx.selectedServer?.name ?? 'Local' }}</span>
        <svg
          class="h-4 w-4 shrink-0 text-slate-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      <Teleport to="body">
        <div v-if="open" class="fixed inset-0 z-40" @click="close" />
      </Teleport>

      <div
        v-if="open"
        class="absolute left-0 z-50 mt-1 w-full rounded-lg bg-slate-800 py-1 shadow-xl ring-1 ring-black/20"
      >
        <button
          v-for="s in serverStore.servers"
          :key="s.id"
          class="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-slate-700"
          :class="s.id === ctx.selectedServerId ? 'text-indigo-400' : 'text-white'"
          @click="pick(s.id)"
        >
          <span class="h-2 w-2 shrink-0 rounded-full" :class="statusColor(s.status)" />
          <span class="truncate">{{ s.name }}</span>
          <span v-if="s.is_local" class="ml-auto text-xs text-slate-500">local</span>
        </button>
      </div>
    </div>
  </div>
</template>
