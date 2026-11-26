<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useRoute } from 'vue-router'
import ServerPicker from './ServerPicker.vue'
import BrandLogo from '@/components/BrandLogo.vue'

const auth = useAuthStore()
const route = useRoute()

defineProps<{ collapsed: boolean }>()
defineEmits<{ (e: 'toggle'): void }>()

interface NavItem {
  path: string
  label: string
  icon: string
  adminOnly?: boolean
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: '',
    items: [
      {
        path: '/dashboard',
        label: 'Dashboard',
        icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6',
      },
    ],
  },
  {
    label: 'Routing',
    items: [
      {
        path: '/proxy-hosts',
        label: 'Proxy Hosts',
        icon: 'M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9',
      },
      {
        path: '/traffic-forwarding',
        label: 'Traffic Forwarding',
        icon: 'M13 10V3L4 14h7v7l9-11h-7z',
      },
      {
        path: '/redirection-hosts',
        label: 'Redirections',
        icon: 'M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z',
      },
      {
        path: '/dead-hosts',
        label: 'Parked Domains',
        icon: 'M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636',
      },
    ],
  },
  {
    label: 'Security',
    items: [
      {
        path: '/certificates',
        label: 'Custom Certs',
        icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
      },
      {
        path: '/access-lists',
        label: 'Access Lists',
        icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z',
      },
    ],
  },
  {
    label: 'System',
    items: [
      {
        path: '/servers',
        label: 'Servers',
        icon: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2',
        adminOnly: true,
      },
      {
        path: '/users',
        label: 'Users',
        icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z',
        adminOnly: true,
      },
      {
        path: '/access-tokens',
        label: 'Access Tokens',
        icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
      },
      {
        path: '/audit-log',
        label: 'Audit Log',
 adminOnly: true,
        icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2',
      },
      {
        path: '/backup',
        label: 'Backup',
        icon: 'M5 8a2 2 0 012-2h4l2 2h4a2 2 0 012 2v6a2 2 0 01-2 2H7a2 2 0 01-2-2V8zm7 3v3m0 0l-2-2m2 2l2-2',
        adminOnly: true,
      },
      {
        path: '/import/nginx',
        label: 'Import Nginx',
        icon: 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12',
      },
    ],
  },
]

function visibleItems(items: NavItem[]) {
  return items.filter((item) => !item.adminOnly || auth.isAdmin)
}

function isActive(path: string) {
  return route.path === path || route.path.startsWith(path + '/')
}
</script>

<template>
  <aside
    class="fixed inset-y-0 left-0 z-30 flex flex-col bg-slate-900 text-white transition-all duration-300"
    :class="collapsed ? 'w-16' : 'w-64'"
  >
    <div
      class="relative flex shrink-0 items-start justify-center"
      :class="collapsed ? 'h-28 pt-2' : 'h-44'"
    >
      <RouterLink to="/dashboard" aria-label="PosternProxy dashboard">
        <BrandLogo dark :class="collapsed ? 'h-14 w-14' : 'h-44 w-44'" />
      </RouterLink>
      <button
        class="absolute rounded p-1 hover:bg-slate-700"
        :class="collapsed ? 'bottom-2 right-5' : 'right-3 top-3'"
        :aria-label="collapsed ? 'Expand menu' : 'Collapse menu'"
        :aria-expanded="!collapsed"
        @click="$emit('toggle')"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 6h16M4 12h16M4 18h16"
          />
        </svg>
      </button>
    </div>

    <div class="px-2 pt-3">
      <ServerPicker v-if="!collapsed" />
    </div>

    <nav class="flex-1 overflow-y-auto px-2 py-3">
      <template v-for="group in navGroups" :key="group.label">
        <template v-if="visibleItems(group.items).length">
          <!-- Group label -->
          <div
            v-if="group.label && !collapsed"
            class="mb-1 mt-4 px-3 text-xs font-semibold uppercase tracking-wider text-slate-500 first:mt-0"
          >
            {{ group.label }}
          </div>
          <div
            v-else-if="group.label && collapsed"
            class="mt-4 border-t border-slate-700/60 first:mt-0"
          />

          <!-- Group items -->
          <div class="space-y-0.5">
            <router-link
              v-for="item in visibleItems(group.items)"
              :key="item.path"
              :to="item.path"
              class="flex items-center rounded-lg px-3 py-2 text-sm transition-colors"
              :class="
                isActive(item.path)
                  ? 'bg-slate-700 text-white'
                  : 'text-slate-300 hover:bg-slate-800 hover:text-white'
              "
              :title="collapsed ? item.label : undefined"
            >
              <svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  :d="item.icon"
                />
              </svg>
              <span v-if="!collapsed" class="ml-3">{{ item.label }}</span>
            </router-link>
          </div>
        </template>
      </template>
    </nav>
  </aside>
</template>
