<script setup lang="ts">
import { ref } from 'vue'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const sidebarCollapsed = ref(false)
</script>

<template>
  <div class="min-h-screen bg-slate-50">
    <AppSidebar :collapsed="sidebarCollapsed" @toggle="sidebarCollapsed = !sidebarCollapsed" />

    <div class="transition-all duration-300" :class="sidebarCollapsed ? 'ml-16' : 'ml-64'">
      <AppHeader>
        <template #title>
          <slot name="title" />
        </template>
      </AppHeader>

      <main class="p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>
