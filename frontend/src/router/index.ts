import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoginView from '@/views/LoginView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { public: true },
    },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', redirect: '/dashboard' },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
        },
        {
          path: 'proxy-hosts',
          name: 'proxy-hosts',
          component: () => import('@/views/proxy-hosts/ProxyHostsView.vue'),
        },
        {
          path: 'redirection-hosts',
          name: 'redirection-hosts',
          component: () => import('@/views/RedirectionHostsView.vue'),
        },
        {
          path: 'traffic-forwarding',
          name: 'traffic-forwarding',
          component: () => import('@/views/TrafficForwardingView.vue'),
        },
        { path: 'stream-hosts', redirect: { name: 'traffic-forwarding' } },
        {
          path: 'dead-hosts',
          name: 'dead-hosts',
          component: () => import('@/views/DeadHostsView.vue'),
        },
        {
          path: 'certificates',
          name: 'certificates',
          component: () => import('@/views/CertificatesView.vue'),
        },
        {
          path: 'access-lists',
          name: 'access-lists',
          component: () => import('@/views/AccessListsView.vue'),
        },
        { path: 'port-forwards', redirect: { name: 'traffic-forwarding' } },
        {
          path: 'servers',
          name: 'servers',
          component: () => import('@/views/ServersView.vue'),
          meta: { adminOnly: true },
        },
        {
          path: 'audit-log',
          name: 'audit-log',
          component: () => import('@/views/AuditLogView.vue'),
        },
        {
          path: 'users',
          name: 'users',
          component: () => import('@/views/UsersView.vue'),
          meta: { adminOnly: true },
        },
        {
          path: 'access-tokens',
          name: 'access-tokens',
          component: () => import('@/views/AccessTokensView.vue'),
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/SettingsView.vue'),
          meta: { adminOnly: true },
        },
        {
          path: 'backup',
          name: 'backup',
          component: () => import('@/views/BackupView.vue'),
          meta: { adminOnly: true },
        },
        {
          path: 'import/nginx',
          name: 'import-nginx',
          component: () => import('@/views/NginxImportView.vue'),
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  // On a fresh page load the in-memory access token is gone. Attempt to
  // restore it from the HttpOnly refresh cookie before deciding the user is
  // logged out. tryRestore() is idempotent and safe to call repeatedly.
  if (!auth.isAuthenticated) {
    await auth.tryRestore()
  }

  if (to.meta.public) return true

  if (!auth.isAuthenticated) {
    return { name: 'login' }
  }

  if (!auth.user) {
    await auth.fetchUser()
    if (!auth.isAuthenticated) {
      return { name: 'login' }
    }
  }

  if (to.meta.adminOnly && !auth.isAdmin) {
    return { name: 'dashboard' }
  }

  return true
})

export default router
