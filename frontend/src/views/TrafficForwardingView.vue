<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import type { StreamHost } from '@/types'
import {
  listStreamHosts,
  createStreamHost,
  updateStreamHost,
  deleteStreamHost,
  enableStreamHost,
  disableStreamHost,
} from '@/api/streamHosts'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'

type ForwardingProtocol = 'tcp' | 'udp' | 'tcp+udp'

interface Rule {
  _id: number
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol: ForwardingProtocol
  enabled: boolean
  server_id: number
  tcp_sni_match?: string | null
}

const serverStore = useServerStore()
const ctx = useServerContext()

const rules = ref<Rule[]>([])
const loading = ref(false)
const error = ref('')

const showModal = ref(false)
const modalMode = ref<'create' | 'edit'>('create')
const editingRule = ref<Rule | null>(null)
const saving = ref(false)
const saveError = ref('')

const showDeleteConfirm = ref(false)
const deletingRule = ref<Rule | null>(null)
const deleting = ref(false)
const deleteError = ref('')

const protocols: ForwardingProtocol[] = ['tcp', 'udp', 'tcp+udp']

const defaultForm = (): {
  server_id: number
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol: ForwardingProtocol
  tcp_sni_match: string
} => ({
  server_id: 1,
  incoming_port: 0,
  forward_host: '',
  forward_port: 0,
  protocol: 'tcp',
  tcp_sni_match: '',
})

const form = reactive(defaultForm())

onMounted(async () => {
  await serverStore.ensureLoaded()
  await fetchRules()
})

watch(() => ctx.selectedServerId, fetchRules)

async function fetchRules() {
  loading.value = true
  error.value = ''
  try {
    const streams = await listStreamHosts(ctx.selectedServerId)
    rules.value = streams
      .map(
        (s: StreamHost): Rule => ({
          _id: s.id,
          incoming_port: s.incoming_port,
          forward_host: s.forward_host,
          forward_port: s.forward_port,
          protocol: s.protocol as ForwardingProtocol,
          enabled: s.enabled,
          server_id: s.server_id,
          tcp_sni_match: s.tcp_sni_match,
        })
      )
      .sort((a, b) => a.incoming_port - b.incoming_port)
  } catch {
    error.value = 'Failed to load forwarding rules.'
  } finally {
    loading.value = false
  }
}

function openCreate() {
  modalMode.value = 'create'
  editingRule.value = null
  saveError.value = ''
  Object.assign(form, { ...defaultForm(), server_id: ctx.selectedServerId })
  showModal.value = true
}

function openEdit(rule: Rule) {
  modalMode.value = 'edit'
  editingRule.value = rule
  saveError.value = ''
  Object.assign(form, {
    server_id: rule.server_id,
    incoming_port: rule.incoming_port,
    forward_host: rule.forward_host,
    forward_port: rule.forward_port,
    protocol: rule.protocol,
    tcp_sni_match: rule.tcp_sni_match ?? '',
  })
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

async function saveForm() {
  if (!form.incoming_port || form.incoming_port < 1) {
    saveError.value = 'Incoming port is required.'
    return
  }
  if (!form.forward_host) {
    saveError.value = 'Forward host is required.'
    return
  }
  if (!form.forward_port || form.forward_port < 1) {
    saveError.value = 'Forward port is required.'
    return
  }
  saving.value = true
  saveError.value = ''
  try {
    const payload = {
      server_id: form.server_id || 1,
      incoming_port: Number(form.incoming_port),
      forward_host: form.forward_host,
      forward_port: Number(form.forward_port),
      protocol: form.protocol,
      tcp_sni_match: form.tcp_sni_match || null,
    }
    if (modalMode.value === 'create') {
      await createStreamHost(payload)
    } else if (editingRule.value) {
      await updateStreamHost(editingRule.value._id, payload)
    }
    await fetchRules()
    closeModal()
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : 'Save failed'
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(rule: Rule) {
  try {
    if (rule.enabled) await disableStreamHost(rule._id)
    else await enableStreamHost(rule._id)
    rule.enabled = !rule.enabled
  } catch {
    /* ignore */
  }
}

function confirmDelete(rule: Rule) {
  deletingRule.value = rule
  deleteError.value = ''
  showDeleteConfirm.value = true
}

async function doDelete() {
  if (!deletingRule.value) return
  deleting.value = true
  deleteError.value = ''
  const target = deletingRule.value
  try {
    await deleteStreamHost(target._id)
    rules.value = rules.value.filter((r) => r !== target)
    showDeleteConfirm.value = false
  } catch (e: unknown) {
    deleteError.value = e instanceof Error ? e.message : 'Failed to delete the rule.'
  } finally {
    deleting.value = false
  }
}

function protocolBadgeClass(proto: string): string {
  const map: Record<string, string> = {
    tcp: 'bg-blue-100 text-blue-700',
    udp: 'bg-violet-100 text-violet-700',
    'tcp+udp': 'bg-indigo-100 text-indigo-700',
  }
  return map[proto] ?? 'bg-slate-100 text-slate-600'
}
</script>

<template>
  <div>
    <!-- Page header -->
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Traffic Forwarding</h2>
        <p class="mt-0.5 text-sm text-slate-500">
          Forward raw TCP or UDP traffic to any backend — databases, MQTT brokers, SSH, game
          servers, and anything that isn't HTTP. Powered by Caddy Layer 4.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="openCreate"
      >
        + Add Rule
      </button>
    </div>

    <!-- Firewall reminder -->
    <div class="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">
      <span class="font-medium">Firewall:</span> each forwarded port must be allowed through the
      server's firewall, e.g. <code class="rounded bg-amber-100 px-1 font-mono">sudo ufw allow 25565/tcp</code>.
      If your server sits behind a cloud firewall (DigitalOcean, AWS, etc.), open the port there too —
      PosternProxy can't manage those.
    </div>

    <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="h-14 animate-pulse rounded-xl bg-slate-100" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!loading && rules.length === 0"
      class="rounded-xl border border-dashed border-slate-300 bg-white p-16 text-center shadow-sm"
    >
      <p class="text-sm font-medium text-slate-500">No forwarding rules configured yet.</p>
      <p class="mt-1 text-xs text-slate-400">
        Forward a TCP/UDP port to any backend — Caddy handles the proxying.
      </p>
    </div>

    <!-- Table -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-slate-200 bg-slate-50">
          <tr>
            <th class="px-4 py-3 font-medium text-slate-600">Port</th>
            <th class="px-4 py-3 font-medium text-slate-600">Forward To</th>
            <th class="px-4 py-3 font-medium text-slate-600">Protocol</th>
            <th class="px-4 py-3 font-medium text-slate-600">Status</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr
            v-for="rule in rules"
            :key="rule._id"
            class="transition-colors hover:bg-slate-50"
            :class="{ 'opacity-50': !rule.enabled }"
          >
            <td class="px-4 py-3 font-mono text-sm font-medium text-slate-800">
              :{{ rule.incoming_port }}
            </td>
            <td class="px-4 py-3">
              <span class="font-mono text-xs text-slate-600"
                >{{ rule.forward_host }}:{{ rule.forward_port }}</span
              >
              <span v-if="rule.tcp_sni_match" class="mt-0.5 block text-xs text-slate-400"
                >SNI: {{ rule.tcp_sni_match }}</span
              >
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium uppercase"
                :class="protocolBadgeClass(rule.protocol)"
              >
                {{ rule.protocol }}
              </span>
            </td>
            <td class="px-4 py-3">
              <button
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors"
                :class="
                  rule.enabled
                    ? 'bg-emerald-100 text-emerald-700 hover:bg-emerald-200'
                    : 'bg-slate-100 text-slate-500 hover:bg-slate-200'
                "
                @click="toggleEnabled(rule)"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="rule.enabled ? 'bg-emerald-500' : 'bg-slate-400'"
                />
                {{ rule.enabled ? 'Enabled' : 'Disabled' }}
              </button>
            </td>
            <td class="px-4 py-3 text-right">
              <div class="flex items-center justify-end gap-2">
                <button
                  class="rounded px-2 py-1 text-xs text-indigo-600 hover:bg-indigo-50"
                  @click="openEdit(rule)"
                >
                  Edit
                </button>
                <button
                  class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
                  @click="confirmDelete(rule)"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Add / Edit Modal ────────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="showModal"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        @click.self="closeModal"
      >
        <div class="flex w-full max-w-lg flex-col rounded-2xl bg-white shadow-2xl">
          <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">
              {{ modalMode === 'create' ? 'Add Forwarding Rule' : 'Edit Forwarding Rule' }}
            </h3>
            <button
              class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
              @click="closeModal"
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

          <div class="space-y-4 overflow-y-auto px-6 py-5" style="max-height: 65vh">
            <div v-if="saveError" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
              {{ saveError }}
            </div>

            <!-- Incoming port -->
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">Incoming Port</label>
              <input
                v-model.number="form.incoming_port"
                type="number"
                min="1"
                max="65535"
                placeholder="8080"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-slate-400">The port to listen on (1–65535)</p>
            </div>

            <!-- Protocol toggle -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">Protocol</label>
              <div class="flex gap-2">
                <button
                  v-for="proto in protocols"
                  :key="proto"
                  type="button"
                  class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
                  :class="
                    form.protocol === proto
                      ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
                      : 'border-slate-200 text-slate-600 hover:border-slate-300'
                  "
                  @click="form.protocol = proto"
                >
                  {{ proto.toUpperCase() }}
                </button>
              </div>
            </div>

            <!-- Forward host + port -->
            <div class="grid grid-cols-12 gap-3">
              <div class="col-span-8">
                <label class="mb-1 block text-sm font-medium text-slate-700">Forward Host</label>
                <input
                  v-model="form.forward_host"
                  type="text"
                  placeholder="192.168.1.50"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div class="col-span-4">
                <label class="mb-1 block text-sm font-medium text-slate-700">Forward Port</label>
                <input
                  v-model.number="form.forward_port"
                  type="number"
                  min="1"
                  max="65535"
                  placeholder="5432"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>

            <!-- SNI match (TLS only) -->
            <div>
              <label class="mb-1 block text-sm font-medium text-slate-700">
                SNI Match
                <span class="text-xs font-normal text-slate-400">(optional, TLS only)</span>
              </label>
              <input
                v-model="form.tcp_sni_match"
                type="text"
                placeholder="db.example.com"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-slate-400">
                Only route TLS connections matching this hostname. Leave blank to forward all
                traffic on the port.
              </p>
            </div>

            <!-- Firewall reminder -->
            <div
              class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800"
            >
              <span class="font-medium">Remember the firewall.</span> After creating this rule, allow
              the port through the server's firewall:
              <code class="rounded bg-amber-100 px-1 font-mono"
                >sudo ufw allow {{ form.incoming_port || 'PORT' }}/{{
                  form.protocol === 'tcp+udp' ? 'tcp && sudo ufw allow ' + (form.incoming_port || 'PORT') + '/udp' : form.protocol
                }}</code
              >. Open the same port on any upstream/cloud firewall too.
            </div>
          </div>

          <div class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-4">
            <button
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              :disabled="saving"
              @click="closeModal"
            >
              Cancel
            </button>
            <button
              class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
              :disabled="saving"
              @click="saveForm"
            >
              {{ saving ? 'Saving…' : modalMode === 'create' ? 'Create Rule' : 'Save Changes' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ── Delete Confirmation ─────────────────────────────────────────────── -->
    <DeleteConfirmDialog
      v-model="showDeleteConfirm"
      title="Delete Forwarding Rule"
      :message="`This will immediately stop forwarding traffic on port ${deletingRule?.incoming_port}. This cannot be undone.`"
      :error="deleteError"
      :busy="deleting"
      @confirm="doDelete"
    />
  </div>
</template>
