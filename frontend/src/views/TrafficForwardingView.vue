<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import type { StreamHost, PortForward } from '@/types'
import {
  listStreamHosts,
  createStreamHost,
  updateStreamHost,
  deleteStreamHost,
  enableStreamHost,
  disableStreamHost,
} from '@/api/streamHosts'
import {
  listPortForwards,
  createPortForward,
  updatePortForward,
  deletePortForward,
  enablePortForward,
  disablePortForward,
  type CreatePortForwardInput,
} from '@/api/portForwards'
import { useServerStore } from '@/stores/servers'
import { useServerContext } from '@/stores/serverContext'
import DeleteConfirmDialog from '@/components/DeleteConfirmDialog.vue'

type ForwardingMethod = 'caddy' | 'iptables' | 'socat'
type ForwardingProtocol = 'tcp' | 'udp' | 'tcp+udp'

interface Rule {
  _id: number
  _source: ForwardingMethod
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol: ForwardingProtocol
  enabled: boolean
  server_id: number
  tcp_sni_match?: string | null
  iface?: string
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

const method = ref<ForwardingMethod>('caddy')
const protocols: ForwardingProtocol[] = ['tcp', 'udp', 'tcp+udp']

const methodCards: { value: ForwardingMethod; label: string; description: string }[] = [
  {
    value: 'caddy',
    label: 'Caddy Layer 4',
    description: "Routes through Caddy's built-in proxy. No extra tools needed.",
  },
  {
    value: 'iptables',
    label: 'iptables',
    description: 'Kernel-level NAT. Fastest, survives Caddy restarts. Requires root.',
  },
  {
    value: 'socat',
    label: 'socat',
    description: 'Userspace relay. Good fallback. No root needed for high ports.',
  },
]

const defaultForm = (): {
  server_id: number
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol: ForwardingProtocol
  tcp_sni_match: string
  iface: string
} => ({
  server_id: 1,
  incoming_port: 0,
  forward_host: '',
  forward_port: 0,
  protocol: 'tcp',
  tcp_sni_match: '',
  iface: '',
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
    const [streams, forwards] = await Promise.all([
      listStreamHosts(ctx.selectedServerId),
      listPortForwards(ctx.selectedServerId),
    ])
    const mapped: Rule[] = [
      ...streams.map(
        (s: StreamHost): Rule => ({
          _id: s.id,
          _source: 'caddy',
          incoming_port: s.incoming_port,
          forward_host: s.forward_host,
          forward_port: s.forward_port,
          protocol: s.protocol,
          enabled: s.enabled,
          server_id: s.server_id,
          tcp_sni_match: s.tcp_sni_match,
        })
      ),
      ...forwards.map(
        (f: PortForward): Rule => ({
          _id: f.id,
          _source: f.method as ForwardingMethod,
          incoming_port: f.incoming_port,
          forward_host: f.forward_host,
          forward_port: f.forward_port,
          protocol: f.protocol,
          enabled: f.enabled,
          server_id: f.server_id,
          iface: f.interface,
        })
      ),
    ]
    mapped.sort((a, b) => a.incoming_port - b.incoming_port)
    rules.value = mapped
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
  method.value = 'caddy'
  Object.assign(form, { ...defaultForm(), server_id: ctx.selectedServerId })
  showModal.value = true
}

function openEdit(rule: Rule) {
  modalMode.value = 'edit'
  editingRule.value = rule
  saveError.value = ''
  method.value = rule._source
  Object.assign(form, {
    server_id: rule.server_id,
    incoming_port: rule.incoming_port,
    forward_host: rule.forward_host,
    forward_port: rule.forward_port,
    protocol: rule.protocol,
    tcp_sni_match: rule.tcp_sni_match ?? '',
    iface: rule.iface ?? '',
  })
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

function setMethod(m: ForwardingMethod) {
  if (modalMode.value === 'edit') return
  method.value = m
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
    if (method.value === 'caddy') {
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
    } else {
      const payload: CreatePortForwardInput = {
        protocol: form.protocol,
        incoming_port: Number(form.incoming_port),
        forward_host: form.forward_host,
        forward_port: Number(form.forward_port),
        interface: form.iface,
        method: method.value,
      }
      if (modalMode.value === 'create') {
        await createPortForward(payload)
      } else if (editingRule.value) {
        await updatePortForward(editingRule.value._id, payload)
      }
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
    if (rule._source === 'caddy') {
      if (rule.enabled) await disableStreamHost(rule._id)
      else await enableStreamHost(rule._id)
    } else {
      if (rule.enabled) await disablePortForward(rule._id)
      else await enablePortForward(rule._id)
    }
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
    if (target._source === 'caddy') {
      await deleteStreamHost(target._id)
    } else {
      await deletePortForward(target._id)
    }
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

function viaBadgeClass(source: ForwardingMethod): string {
  const map: Record<ForwardingMethod, string> = {
    caddy: 'bg-indigo-100 text-indigo-700',
    iptables: 'bg-amber-100 text-amber-700',
    socat: 'bg-teal-100 text-teal-700',
  }
  return map[source]
}

function viaLabel(source: ForwardingMethod): string {
  const map: Record<ForwardingMethod, string> = {
    caddy: 'Caddy L4',
    iptables: 'iptables',
    socat: 'socat',
  }
  return map[source]
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
          servers, and anything that isn't HTTP.
        </p>
      </div>
      <button
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
        @click="openCreate"
      >
        + Add Rule
      </button>
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
        Use Caddy Layer 4 for TLS-aware proxying, or iptables/socat for kernel-level forwarding.
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
            <th class="px-4 py-3 font-medium text-slate-600">Via</th>
            <th class="px-4 py-3 font-medium text-slate-600">Status</th>
            <th class="px-4 py-3 text-right font-medium text-slate-600">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr
            v-for="rule in rules"
            :key="`${rule._source}-${rule._id}`"
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
              <span
                class="rounded px-2 py-0.5 text-xs font-medium"
                :class="viaBadgeClass(rule._source)"
              >
                {{ viaLabel(rule._source) }}
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
                {{ rule.enabled ? 'Active' : 'Inactive' }}
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

            <!-- Method cards -->
            <div>
              <label class="mb-2 block text-sm font-medium text-slate-700">
                Forwarding Method
                <span v-if="modalMode === 'edit'" class="ml-1 text-xs font-normal text-slate-400"
                  >(cannot be changed after creation)</span
                >
              </label>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="m in methodCards"
                  :key="m.value"
                  type="button"
                  class="rounded-xl border-2 p-3 text-left transition-colors"
                  :class="[
                    method === m.value
                      ? 'border-indigo-500 bg-indigo-50'
                      : 'border-slate-200 bg-white hover:border-slate-300',
                    modalMode === 'edit' ? 'cursor-default opacity-70' : 'cursor-pointer',
                  ]"
                  @click="setMethod(m.value)"
                >
                  <div class="text-xs font-semibold text-slate-800">{{ m.label }}</div>
                  <div class="mt-0.5 text-xs leading-snug text-slate-500">{{ m.description }}</div>
                </button>
              </div>
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

            <!-- Caddy L4: SNI match -->
            <div v-if="method === 'caddy'">
              <label class="mb-1 block text-sm font-medium text-slate-700">
                SNI Match
                <span class="text-xs font-normal text-slate-400">(optional)</span>
              </label>
              <input
                v-model="form.tcp_sni_match"
                type="text"
                placeholder="db.example.com"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-slate-400">
                Only route TLS connections matching this hostname. Leave blank to forward all
                traffic.
              </p>
            </div>

            <!-- iptables: network interface -->
            <div v-if="method === 'iptables'">
              <label class="mb-1 block text-sm font-medium text-slate-700">
                Network Interface
                <span class="text-xs font-normal text-slate-400">(optional)</span>
              </label>
              <input
                v-model="form.iface"
                type="text"
                placeholder="eth0"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <p class="mt-1 text-xs text-slate-400">
                Restrict the iptables rule to a specific interface. Leave blank for all interfaces.
              </p>
            </div>

            <!-- Method info banners -->
            <div
              v-if="method === 'iptables'"
              class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800"
            >
              <span class="font-medium">iptables</span> writes DNAT + MASQUERADE rules to the kernel
              NAT table. Requires
              <code class="rounded bg-amber-100 px-0.5 font-mono">CAP_NET_ADMIN</code>. Rules are
              restored automatically on startup.
            </div>
            <div
              v-else-if="method === 'socat'"
              class="rounded-lg border border-teal-200 bg-teal-50 px-4 py-3 text-xs text-teal-700"
            >
              <span class="font-medium">socat</span> spawns a child process on the server. It is
              automatically restarted when the application restarts.
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
