<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()
const showDropdown = ref(false)

function hideDropdown() {
  setTimeout(() => (showDropdown.value = false), 150)
}

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<template>
  <header
    class="sticky top-0 z-20 flex h-16 items-center justify-between border-b border-slate-200 bg-white px-6"
  >
    <h1 class="text-lg font-semibold text-slate-800">
      <slot name="title">Dashboard</slot>
    </h1>

    <div class="relative">
      <button
        class="flex items-center space-x-2 rounded-lg px-3 py-2 text-sm text-slate-600 hover:bg-slate-100"
        @click="showDropdown = !showDropdown"
        @blur="hideDropdown"
      >
        <div
          class="flex h-8 w-8 items-center justify-center rounded-full bg-slate-700 text-white text-xs font-medium"
        >
          {{ auth.user?.name?.charAt(0)?.toUpperCase() || '?' }}
        </div>
        <span>{{ auth.user?.name || 'User' }}</span>
      </button>

      <div
        v-if="showDropdown"
        class="absolute right-0 mt-1 w-48 rounded-lg border border-slate-200 bg-white py-1 shadow-lg"
      >
        <div class="border-b border-slate-100 px-4 py-2 text-xs text-slate-500">
          {{ auth.user?.email }}
        </div>
        <router-link
          to="/settings"
          class="block px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
        >
          Settings
        </router-link>
        <button
          class="block w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50"
          @click="handleLogout"
        >
          Logout
        </button>
      </div>
    </div>
  </header>
</template>
