<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { listTokens, createToken, revokeToken } from '@/api/tokens'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/utils/errors'
import type { APIToken } from '@/api/tokens'

// ── State ─────────────────────────────────────────────────────────────────────
const authStore = useAuthStore()
const tokens = ref<APIToken[]>([])
const loading = ref(true)
const error = ref('')

// ── Create modal ──────────────────────────────────────────────────────────────
const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const form = ref({
  name: '',
  scopes: ['read', 'write'] as string[],
  expires: 'never' as 'never' | '30d' | '90d' | '1y' | 'custom',
  customDate: '',
})

// ── Show-token modal (one-time display after creation) ────────────────────────
const newToken = ref<APIToken | null>(null)
const tokenCopied = ref(false)

// ── Revoke confirmation ───────────────────────────────────────────────────────
const revokeTarget = ref<APIToken | null>(null)
const revoking = ref(false)
const revokeError = ref('')

// ── Computed ──────────────────────────────────────────────────────────────────
const isAdmin = computed(() => authStore.isAdmin)

// Separate human tokens from agent tokens for display
const humanTokens = computed(() => tokens.value.filter((t) => !t.server_id))
const agentTokens = computed(() => tokens.value.filter((t) => !!t.server_id))

// ── Data ──────────────────────────────────────────────────────────────────────
async function fetchTokens() {
  loading.value = true
  error.value = ''
  try {
    tokens.value = await listTokens()
  } catch (e: unknown) {
    error.value = errorMessage(e, 'Failed to load tokens')
  } finally {
    loading.value = false
  }
}

onMounted(fetchTokens)

// ── Create ────────────────────────────────────────────────────────────────────
function openCreate() {
  form.value = { name: '', scopes: ['read', 'write'], expires: 'never', customDate: '' }
  createError.value = ''
  showCreate.value = true
}

function toggleScope(scope: string) {
  const idx = form.value.scopes.indexOf(scope)
  if (scope === 'read') return // read is always required
  if (idx === -1) {
    form.value.scopes.push(scope)
  } else {
    form.value.scopes.splice(idx, 1)
  }
}

function expiresAtFromForm(): string | undefined {
  const now = new Date()
  switch (form.value.expires) {
    case '30d': {
      const d = new Date(now)
      d.setDate(d.getDate() + 30)
      return d.toISOString()
    }
    case '90d': {
      const d = new Date(now)
      d.setDate(d.getDate() + 90)
      return d.toISOString()
    }
    case '1y': {
      const d = new Date(now)
      d.setFullYear(d.getFullYear() + 1)
      return d.toISOString()
    }
    case 'custom':
      return form.value.customDate ? new Date(form.value.customDate).toISOString() : undefined
    default:
      return undefined
  }
}

async function submitCreate() {
  if (!form.value.name.trim()) {
    createError.value = 'Token name is required.'
    return
  }
  creating.value = true
  createError.value = ''
  try {
    const tok = await createToken({
      name: form.value.name.trim(),
      scopes: form.value.scopes,
      expires_at: expiresAtFromForm(),
    })
    showCreate.value = false
    newToken.value = tok // show the raw token once
    tokenCopied.value = false
    await fetchTokens()
  } catch (e: unknown) {
    createError.value = errorMessage(e, 'Failed to create token.')
  } finally {
    creating.value = false
  }
}

// ── Copy token ────────────────────────────────────────────────────────────────
async function copyToken() {
  if (!newToken.value?.token) return
  try {
    await navigator.clipboard.writeText(newToken.value.token)
    tokenCopied.value = true
    setTimeout(() => (tokenCopied.value = false), 2000)
  } catch {
    // clipboard not available — user can select manually
  }
}

// ── Revoke ────────────────────────────────────────────────────────────────────
function openRevoke(token: APIToken) {
  revokeTarget.value = token
  revokeError.value = ''
}

async function confirmRevoke() {
  if (!revokeTarget.value) return
  revoking.value = true
  revokeError.value = ''
  try {
    await revokeToken(revokeTarget.value.id)
    revokeTarget.value = null
    await fetchTokens()
  } catch (e: unknown) {
    revokeError.value = errorMessage(e, 'Failed to revoke token.')
  } finally {
    revoking.value = false
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatDate(iso?: string | null) {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function formatRelative(iso?: string | null) {
  if (!iso) return 'Never'
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'Just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function isExpired(tok: APIToken) {
  return !!tok.expires_at && new Date(tok.expires_at) < new Date()
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Access Tokens</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          Long-lived tokens for CI/CD pipelines, scripts, and API automation.
        </p>
      </div>
      <button
        @click="openCreate"
        class="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 transition-colors"
      >
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 4v16m8-8H4"
          />
        </svg>
        New Token
      </button>
    </div>

    <!-- Info banner -->
    <div
      class="mb-6 rounded-lg border border-indigo-200 bg-indigo-50 px-4 py-3 text-sm text-indigo-800"
    >
      <strong>Usage:</strong> Pass tokens as a Bearer header —
      <code class="rounded bg-indigo-100 px-1.5 py-0.5 font-mono text-xs"
        >Authorization: Bearer pp_tok_…</code
      >
    </div>

    <!-- Error banner -->
    <div
      v-if="error"
      class="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ error }}
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-4">
      <div v-for="i in 3" :key="i" class="h-14 rounded-xl bg-slate-100 animate-pulse"></div>
    </div>

    <template v-else>
      <!-- ── Human / automation tokens ── -->
      <section class="mb-8">
        <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Automation Tokens
        </h3>

        <div
          v-if="humanTokens.length === 0"
          class="rounded-xl border border-dashed border-slate-300 bg-white p-8 text-center"
        >
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
              d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
            />
          </svg>
          <p class="text-sm text-slate-500">
            No automation tokens yet. Create one to use with scripts or CI/CD.
          </p>
        </div>

        <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
          <table class="min-w-full divide-y divide-slate-200">
            <thead class="bg-slate-50">
              <tr>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Name
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Token
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Scopes
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Last Used
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Expires
                </th>
                <th
                  class="px-5 py-3 text-right text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
              <tr
                v-for="tok in humanTokens"
                :key="tok.id"
                class="hover:bg-slate-50 transition-colors"
              >
                <td class="px-5 py-3.5">
                  <span class="text-sm font-medium text-slate-900">{{ tok.name }}</span>
                </td>

                <td class="px-5 py-3.5">
                  <code class="rounded bg-slate-100 px-2 py-0.5 font-mono text-xs text-slate-700">
                    {{ tok.token_prefix }}••••••••
                  </code>
                </td>

                <td class="px-5 py-3.5">
                  <div class="flex flex-wrap gap-1">
                    <span
                      v-for="scope in tok.scopes"
                      :key="scope"
                      class="rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="
                        scope === 'write'
                          ? 'bg-amber-100 text-amber-800'
                          : 'bg-slate-100 text-slate-700'
                      "
                    >
                      {{ scope }}
                    </span>
                  </div>
                </td>

                <td class="px-5 py-3.5 text-sm text-slate-500">
                  {{ formatRelative(tok.last_used_at) }}
                </td>

                <td class="px-5 py-3.5">
                  <span v-if="!tok.expires_at" class="text-sm text-slate-400">Never</span>
                  <span
                    v-else
                    class="text-sm"
                    :class="isExpired(tok) ? 'text-red-600 font-medium' : 'text-slate-700'"
                  >
                    {{ isExpired(tok) ? 'Expired ' : '' }}{{ formatDate(tok.expires_at) }}
                  </span>
                </td>

                <td class="px-5 py-3.5 text-right">
                  <button
                    @click="openRevoke(tok)"
                    class="rounded-md px-3 py-1 text-xs font-medium text-red-500 hover:bg-red-50 hover:text-red-700 transition-colors"
                  >
                    Revoke
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- ── Agent tokens (admin only) ── -->
      <section v-if="isAdmin && agentTokens.length > 0">
        <h3 class="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
          Agent Tokens
          <span class="ml-1.5 normal-case font-normal text-slate-400"
            >(managed automatically by the Servers feature)</span
          >
        </h3>
        <div class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
          <table class="min-w-full divide-y divide-slate-200">
            <thead class="bg-slate-50">
              <tr>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Server
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Token
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Last Used
                </th>
                <th
                  class="px-5 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
                >
                  Created
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200">
              <tr v-for="tok in agentTokens" :key="tok.id" class="hover:bg-slate-50">
                <td class="px-5 py-3.5">
                  <span class="text-sm font-medium text-slate-900">{{ tok.name }}</span>
                </td>
                <td class="px-5 py-3.5">
                  <code class="rounded bg-slate-100 px-2 py-0.5 font-mono text-xs text-slate-700">
                    {{ tok.token_prefix }}••••••••
                  </code>
                </td>
                <td class="px-5 py-3.5 text-sm text-slate-500">
                  {{ formatRelative(tok.last_used_at) }}
                </td>
                <td class="px-5 py-3.5 text-sm text-slate-500">{{ formatDate(tok.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <!-- ════════════════════════════════════════════════════════════
         CREATE TOKEN MODAL
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="showCreate"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
        @click.self="showCreate = false"
      >
        <div class="w-full max-w-md rounded-2xl bg-white shadow-xl">
          <div class="flex items-center justify-between border-b border-slate-100 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">New Access Token</h3>
            <button
              @click="showCreate = false"
              class="rounded-md p-1 text-slate-400 hover:bg-slate-100"
            >
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <div class="px-6 py-5 space-y-5">
            <div
              v-if="createError"
              class="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
            >
              {{ createError }}
            </div>

            <!-- Name -->
            <div>
              <label class="block text-sm font-medium text-slate-700 mb-1">Token Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="e.g. GitHub Actions deploy"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
              />
              <p class="mt-1 text-xs text-slate-400">A description to identify this token later.</p>
            </div>

            <!-- Scopes -->
            <div>
              <label class="block text-sm font-medium text-slate-700 mb-2">Permissions</label>
              <div class="space-y-2">
                <!-- Read — always on, disabled -->
                <label
                  class="flex items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3 cursor-not-allowed"
                >
                  <input
                    type="checkbox"
                    checked
                    disabled
                    class="mt-0.5 h-4 w-4 rounded text-indigo-600"
                  />
                  <div>
                    <p class="text-sm font-medium text-slate-700">Read</p>
                    <p class="text-xs text-slate-500">
                      List and retrieve all resources (always included).
                    </p>
                  </div>
                </label>
                <!-- Write -->
                <label
                  class="flex items-start gap-3 rounded-lg border px-4 py-3 cursor-pointer transition-colors"
                  :class="
                    form.scopes.includes('write')
                      ? 'border-amber-400 bg-amber-50'
                      : 'border-slate-200 hover:border-slate-300'
                  "
                >
                  <input
                    type="checkbox"
                    :checked="form.scopes.includes('write')"
                    @change="toggleScope('write')"
                    class="mt-0.5 h-4 w-4 rounded text-amber-500"
                  />
                  <div>
                    <p class="text-sm font-medium text-slate-700">Write</p>
                    <p class="text-xs text-slate-500">Create, update, and delete resources.</p>
                  </div>
                </label>
              </div>
            </div>

            <!-- Expiry -->
            <div>
              <label class="block text-sm font-medium text-slate-700 mb-2">Expiration</label>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
                <label
                  v-for="opt in [
                    { value: 'never', label: 'Never' },
                    { value: '30d', label: '30 days' },
                    { value: '90d', label: '90 days' },
                    { value: '1y', label: '1 year' },
                  ]"
                  :key="opt.value"
                  class="flex cursor-pointer items-center justify-center rounded-lg border py-2 text-sm transition-colors"
                  :class="
                    form.expires === opt.value
                      ? 'border-indigo-500 bg-indigo-50 font-medium text-indigo-700'
                      : 'border-slate-200 text-slate-600 hover:border-slate-300'
                  "
                >
                  <input type="radio" v-model="form.expires" :value="opt.value" class="sr-only" />
                  {{ opt.label }}
                </label>
              </div>
              <!-- Custom date picker -->
              <div v-if="form.expires === 'custom'" class="mt-2">
                <input
                  v-model="form.customDate"
                  type="date"
                  :min="new Date().toISOString().split('T')[0]"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>
            </div>
          </div>

          <div class="flex justify-end gap-3 border-t border-slate-100 px-6 py-4">
            <button
              @click="showCreate = false"
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="submitCreate"
              :disabled="creating || !form.name.trim()"
              class="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="creating" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
              {{ creating ? 'Creating…' : 'Create Token' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ════════════════════════════════════════════════════════════
         ONE-TIME TOKEN DISPLAY MODAL
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="newToken"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
      >
        <div class="w-full max-w-lg rounded-2xl bg-white shadow-xl">
          <div class="border-b border-slate-100 px-6 py-4 text-center">
            <div
              class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100"
            >
              <svg
                class="h-6 w-6 text-emerald-600"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </div>
            <h3 class="text-base font-semibold text-slate-800">Token Created</h3>
            <p class="mt-1 text-sm text-slate-500">
              Copy it now — this is the only time it will be shown.
            </p>
          </div>

          <div class="px-6 py-5">
            <div class="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <p class="mb-1.5 text-xs font-medium text-slate-500">{{ newToken.name }}</p>
              <div class="flex items-center gap-2">
                <code
                  class="flex-1 break-all rounded bg-white border border-slate-200 px-3 py-2 font-mono text-sm text-slate-900 select-all"
                >
                  {{ newToken.token }}
                </code>
                <button
                  @click="copyToken"
                  class="shrink-0 rounded-lg border border-slate-300 p-2 text-slate-600 hover:bg-slate-100 transition-colors"
                  :title="tokenCopied ? 'Copied!' : 'Copy to clipboard'"
                >
                  <svg
                    v-if="!tokenCopied"
                    class="h-4 w-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                    />
                  </svg>
                  <svg
                    v-else
                    class="h-4 w-4 text-emerald-600"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                </button>
              </div>
            </div>

            <div
              class="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
            >
              ⚠️ Store this token securely. It cannot be retrieved again. If you lose it, revoke it
              and create a new one.
            </div>
          </div>

          <div class="flex justify-end border-t border-slate-100 px-6 py-4">
            <button
              @click="newToken = null"
              class="rounded-lg bg-slate-900 px-5 py-2 text-sm font-medium text-white hover:bg-slate-800 transition-colors"
            >
              Done, I've saved it
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ════════════════════════════════════════════════════════════
         REVOKE CONFIRMATION MODAL
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="revokeTarget"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
        @click.self="revokeTarget = null"
      >
        <div class="w-full max-w-sm rounded-2xl bg-white shadow-xl">
          <div class="px-6 pt-6 pb-4 text-center">
            <div
              class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-100"
            >
              <svg
                class="h-6 w-6 text-red-600"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                />
              </svg>
            </div>
            <h3 class="text-base font-semibold text-slate-800">Revoke token?</h3>
            <p class="mt-1.5 text-sm text-slate-500">
              <span class="font-medium text-slate-700">{{ revokeTarget?.name }}</span>
              will stop working immediately. Any scripts or integrations using it will lose access.
            </p>
          </div>

          <div
            v-if="revokeError"
            class="mx-6 mb-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
          >
            {{ revokeError }}
          </div>

          <div class="flex gap-3 border-t border-slate-100 px-6 py-4">
            <button
              @click="revokeTarget = null"
              class="flex-1 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="confirmRevoke"
              :disabled="revoking"
              class="flex-1 inline-flex items-center justify-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="revoking" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
              {{ revoking ? 'Revoking…' : 'Revoke Token' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
