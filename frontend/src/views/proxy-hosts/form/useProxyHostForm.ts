import { inject, provide, reactive, ref, type InjectionKey, type Ref } from 'vue'
import type { ProxyHost, ProxyHostLocation, CustomHeaderRule, ExtraUpstream } from '@/types'
import { createProxyHost, updateProxyHost, type CreateProxyHostInput } from '@/api/proxyHosts'

// ── Public types ─────────────────────────────────────────────────────────────

export type TabId =
  | 'details'
  | 'upstreams'
  | 'locations'
  | 'ssl'
  | 'health'
  | 'security'
  | 'errors'
  | 'headers'
  | 'advanced'

export type SslMode = 'http' | 'auto' | 'https'

export type ProxyHostForm = CreateProxyHostInput & {
  domainInput: string
  server_id: number
  rate_limit_enabled: boolean
  rate_limit_requests: number
  rate_limit_window: string
  tls_passthrough: boolean
  http_only: boolean
  error_pages: Record<string, string>
  custom_headers: CustomHeaderRule[]
  extra_upstreams: ExtraUpstream[]
  lb_policy: string
}

export interface ProxyHostFormAPI {
  // Shared reactive state
  form: ProxyHostForm
  saving: Ref<boolean>
  saveError: Ref<string>
  activeTab: Ref<TabId>
  showAdvanced: Ref<boolean>
  sslMode: Ref<SslMode>

  // SSL helpers
  setSslMode: (m: SslMode) => void

  // Domain tag-input
  addDomain: () => void
  removeDomain: (i: number) => void
  domainKeydown: (e: KeyboardEvent) => void

  // Locations
  addLocation: () => void
  removeLocation: (i: number) => void

  // Extra upstreams
  addExtraUpstream: () => void
  removeExtraUpstream: (i: number) => void

  // Custom header rules
  addHeaderRule: () => void
  removeHeaderRule: (i: number) => void

  // Modal-shell-only operations
  init: (host: ProxyHost | null, defaultServerId: number) => void
  save: (existingHost: ProxyHost | null) => Promise<ProxyHost>
}

// ── Provide / inject machinery ───────────────────────────────────────────────

const KEY: InjectionKey<ProxyHostFormAPI> = Symbol('ProxyHostFormAPI')

/**
 * Called by the modal shell exactly once. Builds the form API, makes it
 * available to descendant tab components via provide(), and returns it for
 * the shell's own use.
 */
export function provideProxyHostForm(): ProxyHostFormAPI {
  const api = createProxyHostFormAPI()
  provide(KEY, api)
  return api
}

/**
 * Called by tab components (DetailsTab, HealthTab, etc.) to access the
 * shared form state. Throws if called outside a ProxyHostFormModal.
 */
export function useProxyHostForm(): ProxyHostFormAPI {
  const api = inject(KEY)
  if (!api) {
    throw new Error(
      'useProxyHostForm() must be called inside a component descended from ProxyHostFormModal'
    )
  }
  return api
}

// ── Factory (private) ────────────────────────────────────────────────────────

function defaultForm(): ProxyHostForm {
  return {
    domainInput: '',
    domain_names: [],
    server_id: 1,
    forward_scheme: 'http',
    forward_host: '',
    forward_port: 80,
    ssl_forced: false,
    hsts_enabled: false,
    hsts_subdomains: false,
    http2_support: true,
    websocket_support: false,
    certificate_id: null,
    access_list_id: null,
    advanced_config: '',
    locations: [],
    hc_enabled: false,
    hc_active_path: '/health',
    hc_active_interval: 30,
    hc_active_timeout: 5,
    hc_active_expect_status: 200,
    hc_passive_fail_duration: 30,
    hc_passive_max_fails: 3,
    hc_passive_unhealthy_latency: 0,
    rate_limit_enabled: false,
    rate_limit_requests: 100,
    rate_limit_window: '1m',
    tls_passthrough: false,
    http_only: false,
    error_pages: { '502': '', '503': '' },
    custom_headers: [],
    extra_upstreams: [],
    lb_policy: 'random',
  }
}

function createProxyHostFormAPI(): ProxyHostFormAPI {
  const form = reactive(defaultForm())
  const saving = ref(false)
  const saveError = ref('')
  const activeTab = ref<TabId>('details')
  const showAdvanced = ref(false)
  const sslMode = ref<SslMode>('auto')

  // ── SSL mode ──────────────────────────────────────────────────────────────

  function initSslMode() {
    if (form.http_only) sslMode.value = 'http'
    else if (form.certificate_id) sslMode.value = 'https'
    else sslMode.value = 'auto'
  }

  function setSslMode(m: SslMode) {
    sslMode.value = m
    if (m === 'http') {
      form.http_only = true
      form.certificate_id = null
      form.hsts_enabled = false
      form.hsts_subdomains = false
    } else if (m === 'auto') {
      form.http_only = false
      form.certificate_id = null
    } else {
      form.http_only = false
      // certificate_id left for user to choose from the dropdown
    }
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  function addDomain() {
    const v = form.domainInput.trim()
    if (v && !form.domain_names.includes(v)) {
      form.domain_names.push(v)
    }
    form.domainInput = ''
  }

  function removeDomain(i: number) {
    form.domain_names.splice(i, 1)
  }

  function domainKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      addDomain()
    } else if (e.key === 'Backspace' && form.domainInput === '' && form.domain_names.length) {
      form.domain_names.pop()
    }
  }

  function addLocation() {
    form.locations = form.locations ?? []
    ;(form.locations as ProxyHostLocation[]).push({
      path: '/',
      forward_scheme: 'http',
      forward_host: '',
      forward_port: 80,
      advanced_config: '',
    })
  }

  function removeLocation(i: number) {
    ;(form.locations as ProxyHostLocation[]).splice(i, 1)
  }

  function addExtraUpstream() {
    form.extra_upstreams.push({ host: '', port: 80 })
  }

  function removeExtraUpstream(i: number) {
    form.extra_upstreams.splice(i, 1)
  }

  function addHeaderRule() {
    form.custom_headers.push({ direction: 'request', operation: 'set', name: '', value: '' })
  }

  function removeHeaderRule(i: number) {
    form.custom_headers.splice(i, 1)
  }

  // ── Modal-shell-only operations ───────────────────────────────────────────

  function init(host: ProxyHost | null, defaultServerId: number) {
    activeTab.value = 'details'
    saveError.value = ''

    if (host === null) {
      Object.assign(form, { ...defaultForm(), server_id: defaultServerId })
      showAdvanced.value = false
    } else {
      Object.assign(form, {
        domainInput: '',
        domain_names: [...host.domain_names],
        server_id: host.server_id ?? 1,
        forward_scheme: host.forward_scheme,
        forward_host: host.forward_host,
        forward_port: host.forward_port,
        ssl_forced: host.ssl_forced,
        hsts_enabled: host.hsts_enabled,
        hsts_subdomains: host.hsts_subdomains,
        http2_support: host.http2_support,
        websocket_support: host.websocket_support,
        certificate_id: host.certificate_id ?? null,
        access_list_id: host.access_list_id ?? null,
        advanced_config: host.advanced_config ?? '',
        locations: host.locations ? host.locations.map((l: ProxyHostLocation) => ({ ...l })) : [],
        hc_enabled: host.hc_enabled ?? false,
        hc_active_path: host.hc_active_path || '/health',
        hc_active_interval: host.hc_active_interval || 30,
        hc_active_timeout: host.hc_active_timeout || 5,
        hc_active_expect_status: host.hc_active_expect_status || 200,
        hc_passive_fail_duration: host.hc_passive_fail_duration || 30,
        hc_passive_max_fails: host.hc_passive_max_fails || 3,
        hc_passive_unhealthy_latency: host.hc_passive_unhealthy_latency || 0,
        rate_limit_enabled: host.rate_limit_enabled ?? false,
        rate_limit_requests: host.rate_limit_requests || 100,
        rate_limit_window: host.rate_limit_window || '1m',
        tls_passthrough: host.tls_passthrough ?? false,
        http_only: host.http_only ?? false,
        error_pages: {
          '502': host.error_pages?.['502'] ?? '',
          '503': host.error_pages?.['503'] ?? '',
        },
        custom_headers: (host.custom_headers ?? []).map((r) => ({ ...r })),
        extra_upstreams: (host.extra_upstreams ?? []).map((u) => ({ ...u })),
        lb_policy: host.lb_policy || 'random',
      })
      showAdvanced.value =
        (host.extra_upstreams?.length ?? 0) > 0 ||
        (host.locations?.length ?? 0) > 0 ||
        host.hc_enabled === true ||
        host.rate_limit_enabled === true ||
        host.tls_passthrough === true ||
        Object.values(host.error_pages ?? {}).some((v) => v !== '') ||
        (host.custom_headers?.length ?? 0) > 0 ||
        (host.advanced_config ?? '').trim() !== ''
    }
    initSslMode()
  }

  async function save(existingHost: ProxyHost | null): Promise<ProxyHost> {
    if (form.domain_names.length === 0) {
      saveError.value = 'At least one domain name is required.'
      activeTab.value = 'details'
      throw new Error('validation')
    }
    if (!form.forward_host) {
      saveError.value = 'Forward hostname is required.'
      activeTab.value = 'details'
      throw new Error('validation')
    }

    saving.value = true
    saveError.value = ''
    try {
      const payload: CreateProxyHostInput = {
        domain_names: form.domain_names,
        server_id: form.server_id || 1,
        forward_scheme: form.forward_scheme,
        forward_host: form.forward_host,
        forward_port: Number(form.forward_port),
        ssl_forced: form.ssl_forced,
        hsts_enabled: form.hsts_enabled,
        hsts_subdomains: form.hsts_subdomains,
        http2_support: form.http2_support,
        websocket_support: form.websocket_support,
        certificate_id: form.certificate_id ?? null,
        access_list_id: form.access_list_id ?? null,
        advanced_config: form.advanced_config,
        locations: form.locations,
        hc_enabled: form.hc_enabled,
        hc_active_path: form.hc_active_path,
        hc_active_interval: Number(form.hc_active_interval) || 0,
        hc_active_timeout: Number(form.hc_active_timeout) || 0,
        hc_active_expect_status: Number(form.hc_active_expect_status) || 0,
        hc_passive_fail_duration: Number(form.hc_passive_fail_duration) || 0,
        hc_passive_max_fails: Number(form.hc_passive_max_fails) || 0,
        hc_passive_unhealthy_latency: Number(form.hc_passive_unhealthy_latency) || 0,
        rate_limit_enabled: form.rate_limit_enabled,
        rate_limit_requests: Number(form.rate_limit_requests) || 100,
        rate_limit_window: form.rate_limit_window || '1m',
        tls_passthrough: form.tls_passthrough,
        http_only: form.http_only,
        error_pages: Object.fromEntries(
          Object.entries(form.error_pages).filter(([, v]) => v.trim() !== '')
        ),
        custom_headers: form.custom_headers.filter((r) => r.name.trim() !== ''),
        extra_upstreams: form.extra_upstreams.filter((u) => u.host.trim() !== ''),
        lb_policy: form.lb_policy || 'random',
      }
      return existingHost === null
        ? await createProxyHost(payload)
        : await updateProxyHost(existingHost.id, payload)
    } catch (e: unknown) {
      saveError.value = e instanceof Error ? e.message : 'Save failed'
      throw e
    } finally {
      saving.value = false
    }
  }

  return {
    form,
    saving,
    saveError,
    activeTab,
    showAdvanced,
    sslMode,
    setSslMode,
    addDomain,
    removeDomain,
    domainKeydown,
    addLocation,
    removeLocation,
    addExtraUpstream,
    removeExtraUpstream,
    addHeaderRule,
    removeHeaderRule,
    init,
    save,
  }
}
