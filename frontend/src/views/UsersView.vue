<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { listUsers, createUser, updateUser, deleteUser, changePassword } from '@/api/users'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/utils/errors'
import type { User } from '@/types'

// ── State ─────────────────────────────────────────────────────────────────────
const authStore = useAuthStore()
const users = ref<User[]>([])
const loading = ref(true)
const error = ref('')

// ── Modal state ───────────────────────────────────────────────────────────────
type ModalMode = 'create' | 'edit' | 'password' | 'delete' | null
const modalMode = ref<ModalMode>(null)
const modalTarget = ref<User | null>(null)
const modalSaving = ref(false)
const modalError = ref('')

// Create / Edit profile form
const form = ref({
  name: '',
  email: '',
  role: 'viewer' as 'admin' | 'viewer',
  is_active: true,
  password: '', // only used on create
})

// Password change form
const pwForm = ref({
  password: '',
  confirm: '',
})

// Active tab inside create/edit modal
const activeTab = ref<'profile' | 'password'>('profile')

// ── Computed ──────────────────────────────────────────────────────────────────
const isEditing = computed(() => modalMode.value === 'edit')
const isCreating = computed(() => modalMode.value === 'create')
const meId = computed(() => authStore.user?.id ?? -1)

// ── Data loading ──────────────────────────────────────────────────────────────
async function fetchUsers() {
  loading.value = true
  error.value = ''
  try {
    users.value = await listUsers()
  } catch (e: unknown) {
    error.value = errorMessage(e, 'Failed to load users')
  } finally {
    loading.value = false
  }
}

onMounted(fetchUsers)

// ── Open modals ───────────────────────────────────────────────────────────────
function openCreate() {
  form.value = { name: '', email: '', role: 'viewer', is_active: true, password: '' }
  activeTab.value = 'profile'
  modalTarget.value = null
  modalError.value = ''
  modalMode.value = 'create'
}

function openEdit(user: User) {
  form.value = {
    name: user.name,
    email: user.email,
    role: user.role,
    is_active: user.is_active,
    password: '',
  }
  activeTab.value = 'profile'
  modalTarget.value = user
  modalError.value = ''
  modalMode.value = 'edit'
}

function openPassword(user: User) {
  pwForm.value = { password: '', confirm: '' }
  modalTarget.value = user
  modalError.value = ''
  modalMode.value = 'password'
}

function openDelete(user: User) {
  modalTarget.value = user
  modalError.value = ''
  modalMode.value = 'delete'
}

function closeModal() {
  modalMode.value = null
  modalTarget.value = null
  modalError.value = ''
  modalSaving.value = false
}

// ── Actions ───────────────────────────────────────────────────────────────────
async function saveUser() {
  modalSaving.value = true
  modalError.value = ''
  try {
    if (isCreating.value) {
      if (!form.value.password) {
        modalError.value = 'Password is required when creating a user.'
        return
      }
      await createUser({
        name: form.value.name.trim(),
        email: form.value.email.trim(),
        role: form.value.role,
        password: form.value.password,
      })
    } else {
      await updateUser(modalTarget.value!.id, {
        name: form.value.name.trim(),
        email: form.value.email.trim(),
        role: form.value.role,
        is_active: form.value.is_active,
      })
    }
    closeModal()
    await fetchUsers()
  } catch (e: unknown) {
    modalError.value = errorMessage(e, 'Save failed.')
  } finally {
    modalSaving.value = false
  }
}

async function savePassword() {
  if (pwForm.value.password !== pwForm.value.confirm) {
    modalError.value = 'Passwords do not match.'
    return
  }
  if (pwForm.value.password.length < 8) {
    modalError.value = 'Password must be at least 8 characters.'
    return
  }
  modalSaving.value = true
  modalError.value = ''
  try {
    await changePassword(modalTarget.value!.id, pwForm.value.password)
    closeModal()
  } catch (e: unknown) {
    modalError.value = errorMessage(e, 'Failed to change password.')
  } finally {
    modalSaving.value = false
  }
}

async function confirmDelete() {
  modalSaving.value = true
  modalError.value = ''
  try {
    await deleteUser(modalTarget.value!.id)
    closeModal()
    await fetchUsers()
  } catch (e: unknown) {
    modalError.value = errorMessage(e, 'Delete failed.')
  } finally {
    modalSaving.value = false
  }
}

async function toggleActive(user: User) {
  try {
    await updateUser(user.id, { is_active: !user.is_active })
    await fetchUsers()
  } catch (e: unknown) {
    error.value = errorMessage(e, 'Failed to update user.')
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}
</script>

<template>
  <div>
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-xl font-semibold text-slate-800">Users</h2>
        <p class="mt-0.5 text-sm text-slate-500">Manage who can access PosternProxy.</p>
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
        Add User
      </button>
    </div>

    <!-- Global error -->
    <div
      v-if="error"
      class="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
    >
      {{ error }}
    </div>

    <!-- Loading skeleton -->
    <div
      v-if="loading"
      class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm animate-pulse"
    >
      <div class="divide-y divide-slate-100">
        <div v-for="i in 4" :key="i" class="flex items-center gap-4 px-6 py-4">
          <div class="h-8 w-8 rounded-full bg-slate-200 shrink-0"></div>
          <div class="flex-1 space-y-1.5">
            <div class="h-3.5 w-32 rounded bg-slate-200"></div>
            <div class="h-3 w-48 rounded bg-slate-100"></div>
          </div>
          <div class="h-5 w-14 rounded-full bg-slate-200"></div>
          <div class="h-5 w-12 rounded-full bg-slate-200"></div>
          <div class="h-4 w-20 rounded bg-slate-100"></div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="users.length === 0"
      class="rounded-xl border border-slate-200 bg-white p-12 text-center shadow-sm"
    >
      <svg
        class="mx-auto h-10 w-10 text-slate-300 mb-3"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M17 20h5v-2a4 4 0 00-5-3.87M9 20H4v-2a4 4 0 015-3.87m6-4a4 4 0 11-8 0 4 4 0 018 0zm6 0a3 3 0 11-6 0 3 3 0 016 0z"
        />
      </svg>
      <p class="text-slate-500 font-medium">No users yet</p>
      <p class="text-sm text-slate-400 mt-1">Click "Add User" to create the first one.</p>
    </div>

    <!-- Users table -->
    <div v-else class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <table class="min-w-full divide-y divide-slate-200">
        <thead class="bg-slate-50">
          <tr>
            <th
              class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
            >
              User
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
            >
              Role
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
            >
              Status
            </th>
            <th
              class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-slate-500"
            >
              Created
            </th>
            <th
              class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-slate-500"
            >
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200">
          <tr
            v-for="user in users"
            :key="user.id"
            class="hover:bg-slate-50 transition-colors"
            :class="{ 'opacity-60': !user.is_active }"
          >
            <!-- User identity -->
            <td class="px-6 py-4">
              <div class="flex items-center gap-3">
                <!-- Avatar -->
                <div
                  class="h-8 w-8 rounded-full flex items-center justify-center text-xs font-bold text-white shrink-0"
                  :class="user.role === 'admin' ? 'bg-purple-500' : 'bg-slate-400'"
                >
                  {{ user.name.charAt(0).toUpperCase() }}
                </div>
                <div>
                  <div class="flex items-center gap-1.5">
                    <span class="text-sm font-medium text-slate-900">{{ user.name }}</span>
                    <!-- "You" badge -->
                    <span
                      v-if="user.id === meId"
                      class="text-[10px] font-semibold rounded bg-indigo-100 text-indigo-700 px-1.5 py-0.5 leading-none"
                    >
                      you
                    </span>
                  </div>
                  <div class="text-xs text-slate-500">{{ user.email }}</div>
                </div>
              </div>
            </td>

            <!-- Role badge -->
            <td class="whitespace-nowrap px-6 py-4">
              <span
                class="inline-flex rounded-full px-2.5 py-0.5 text-xs font-semibold leading-5"
                :class="
                  user.role === 'admin'
                    ? 'bg-purple-100 text-purple-800'
                    : 'bg-slate-100 text-slate-700'
                "
              >
                {{ user.role === 'admin' ? '⚡ Admin' : 'Viewer' }}
              </span>
            </td>

            <!-- Status badge + toggle -->
            <td class="whitespace-nowrap px-6 py-4">
              <button
                v-if="user.id !== meId"
                @click="toggleActive(user)"
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold leading-5 transition-colors"
                :class="
                  user.is_active
                    ? 'bg-emerald-100 text-emerald-800 hover:bg-emerald-200'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                "
                :title="user.is_active ? 'Click to disable' : 'Click to enable'"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="user.is_active ? 'bg-emerald-500' : 'bg-slate-400'"
                ></span>
                {{ user.is_active ? 'Active' : 'Disabled' }}
              </button>
              <!-- Self: non-clickable status -->
              <span
                v-else
                class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold leading-5 bg-emerald-100 text-emerald-800"
              >
                <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                Active
              </span>
            </td>

            <!-- Created date -->
            <td class="whitespace-nowrap px-6 py-4 text-sm text-slate-500">
              {{ formatDate(user.created_at) }}
            </td>

            <!-- Actions -->
            <td class="whitespace-nowrap px-6 py-4 text-right">
              <div class="flex items-center justify-end gap-2">
                <!-- Edit -->
                <button
                  @click="openEdit(user)"
                  class="rounded-md px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900 transition-colors"
                >
                  Edit
                </button>
                <!-- Change password -->
                <button
                  @click="openPassword(user)"
                  class="rounded-md px-3 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900 transition-colors"
                >
                  Password
                </button>
                <!-- Delete — disabled for self -->
                <button
                  v-if="user.id !== meId"
                  @click="openDelete(user)"
                  class="rounded-md px-3 py-1 text-xs font-medium text-red-500 hover:bg-red-50 hover:text-red-700 transition-colors"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ════════════════════════════════════════════════════════════
         CREATE / EDIT MODAL
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="modalMode === 'create' || modalMode === 'edit'"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
        @click.self="closeModal"
      >
        <div class="w-full max-w-md rounded-2xl bg-white shadow-xl">
          <!-- Modal header -->
          <div class="flex items-center justify-between border-b border-slate-100 px-6 py-4">
            <h3 class="text-base font-semibold text-slate-800">
              {{ isCreating ? 'Add User' : 'Edit User' }}
            </h3>
            <button
              @click="closeModal"
              class="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
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

          <!-- Tabs — edit mode only shows Profile; create has no password tab (done inline) -->
          <div v-if="isEditing" class="flex border-b border-slate-100 px-6 gap-6">
            <button
              v-for="tab in ['profile', 'password'] as const"
              :key="tab"
              @click="activeTab = tab"
              class="py-3 text-sm font-medium border-b-2 transition-colors capitalize"
              :class="
                activeTab === tab
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700'
              "
            >
              {{ tab === 'profile' ? 'Profile' : 'Change Password' }}
            </button>
          </div>

          <!-- Body -->
          <div class="px-6 py-5 space-y-4">
            <!-- Error banner -->
            <div
              v-if="modalError"
              class="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
            >
              {{ modalError }}
            </div>

            <!-- ── Profile tab (or create form) ── -->
            <template v-if="!isEditing || activeTab === 'profile'">
              <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">Full Name</label>
                <input
                  v-model="form.name"
                  type="text"
                  placeholder="Jane Smith"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>

              <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">Email</label>
                <input
                  v-model="form.email"
                  type="email"
                  placeholder="jane@example.com"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>

              <!-- Password field — create only -->
              <div v-if="isCreating">
                <label class="block text-sm font-medium text-slate-700 mb-1">Password</label>
                <input
                  v-model="form.password"
                  type="password"
                  placeholder="Min. 8 characters"
                  autocomplete="new-password"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>

              <!-- Role -->
              <div>
                <label class="block text-sm font-medium text-slate-700 mb-2">Role</label>
                <div class="grid grid-cols-2 gap-3">
                  <label
                    class="flex cursor-pointer flex-col gap-1 rounded-lg border p-3 transition-colors"
                    :class="
                      form.role === 'viewer'
                        ? 'border-indigo-500 bg-indigo-50'
                        : 'border-slate-200 hover:border-slate-300'
                    "
                  >
                    <input type="radio" v-model="form.role" value="viewer" class="sr-only" />
                    <span class="text-sm font-medium text-slate-800">Viewer</span>
                    <span class="text-xs text-slate-500">Read-only access to all resources.</span>
                  </label>
                  <label
                    class="flex cursor-pointer flex-col gap-1 rounded-lg border p-3 transition-colors"
                    :class="
                      form.role === 'admin'
                        ? 'border-purple-500 bg-purple-50'
                        : 'border-slate-200 hover:border-slate-300'
                    "
                  >
                    <input type="radio" v-model="form.role" value="admin" class="sr-only" />
                    <span class="text-sm font-medium text-slate-800">Admin</span>
                    <span class="text-xs text-slate-500"
                      >Full access, including user management.</span
                    >
                  </label>
                </div>
              </div>

              <!-- Active toggle — edit only, not self -->
              <div
                v-if="isEditing && modalTarget && modalTarget.id !== meId"
                class="flex items-center justify-between rounded-lg border border-slate-200 px-4 py-3"
              >
                <div>
                  <p class="text-sm font-medium text-slate-700">Account active</p>
                  <p class="text-xs text-slate-500">Disabled users cannot log in.</p>
                </div>
                <button
                  type="button"
                  @click="form.is_active = !form.is_active"
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors"
                  :class="form.is_active ? 'bg-indigo-600' : 'bg-slate-200'"
                >
                  <span
                    class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition-transform"
                    :class="form.is_active ? 'translate-x-5' : 'translate-x-0'"
                  ></span>
                </button>
              </div>
            </template>

            <!-- ── Password tab (edit mode only) ── -->
            <template v-if="isEditing && activeTab === 'password'">
              <div
                class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
              >
                Setting a new password will immediately invalidate the user's current session.
              </div>

              <div>
                <label class="block text-sm font-medium text-slate-700 mb-1">New Password</label>
                <input
                  v-model="pwForm.password"
                  type="password"
                  placeholder="Min. 8 characters"
                  autocomplete="new-password"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>

              <div>
                <label class="block text-sm font-medium text-slate-700 mb-1"
                  >Confirm Password</label
                >
                <input
                  v-model="pwForm.confirm"
                  type="password"
                  placeholder="Repeat new password"
                  autocomplete="new-password"
                  class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
                />
              </div>
            </template>
          </div>

          <!-- Footer -->
          <div class="flex justify-end gap-3 border-t border-slate-100 px-6 py-4">
            <button
              @click="closeModal"
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>

            <!-- Save profile (create or edit profile tab) -->
            <button
              v-if="!isEditing || activeTab === 'profile'"
              @click="saveUser"
              :disabled="modalSaving"
              class="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="modalSaving" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
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
              {{ modalSaving ? 'Saving…' : isCreating ? 'Create User' : 'Save Changes' }}
            </button>

            <!-- Save password (edit password tab) -->
            <button
              v-if="isEditing && activeTab === 'password'"
              @click="savePassword"
              :disabled="modalSaving || !pwForm.password || !pwForm.confirm"
              class="inline-flex items-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="modalSaving" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
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
              {{ modalSaving ? 'Saving…' : 'Set Password' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ════════════════════════════════════════════════════════════
         STANDALONE CHANGE PASSWORD MODAL (from table action button)
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="modalMode === 'password'"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
        @click.self="closeModal"
      >
        <div class="w-full max-w-md rounded-2xl bg-white shadow-xl">
          <div class="flex items-center justify-between border-b border-slate-100 px-6 py-4">
            <div>
              <h3 class="text-base font-semibold text-slate-800">Change Password</h3>
              <p class="text-xs text-slate-500 mt-0.5">
                {{ modalTarget?.name }} · {{ modalTarget?.email }}
              </p>
            </div>
            <button
              @click="closeModal"
              class="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
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

          <div class="px-6 py-5 space-y-4">
            <div
              v-if="modalError"
              class="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
            >
              {{ modalError }}
            </div>

            <div
              class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
            >
              Setting a new password will immediately invalidate the user's current session.
            </div>

            <div>
              <label class="block text-sm font-medium text-slate-700 mb-1">New Password</label>
              <input
                v-model="pwForm.password"
                type="password"
                placeholder="Min. 8 characters"
                autocomplete="new-password"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-slate-700 mb-1">Confirm Password</label>
              <input
                v-model="pwForm.confirm"
                type="password"
                placeholder="Repeat new password"
                autocomplete="new-password"
                class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 outline-none"
              />
            </div>
          </div>

          <div class="flex justify-end gap-3 border-t border-slate-100 px-6 py-4">
            <button
              @click="closeModal"
              class="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="savePassword"
              :disabled="modalSaving || !pwForm.password || !pwForm.confirm"
              class="inline-flex items-center gap-2 rounded-lg bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="modalSaving" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
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
              {{ modalSaving ? 'Saving…' : 'Set Password' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ════════════════════════════════════════════════════════════
         DELETE CONFIRMATION MODAL
    ════════════════════════════════════════════════════════════ -->
    <Teleport to="body">
      <div
        v-if="modalMode === 'delete'"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4"
        @click.self="closeModal"
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
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4h6v3M4 7h16"
                />
              </svg>
            </div>
            <h3 class="text-base font-semibold text-slate-800">Delete user?</h3>
            <p class="mt-1.5 text-sm text-slate-500">
              <span class="font-medium text-slate-700">{{ modalTarget?.name }}</span>
              ({{ modalTarget?.email }}) will be permanently removed. This cannot be undone.
            </p>
          </div>

          <div
            v-if="modalError"
            class="mx-6 mb-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700"
          >
            {{ modalError }}
          </div>

          <div class="flex gap-3 border-t border-slate-100 px-6 py-4">
            <button
              @click="closeModal"
              class="flex-1 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Cancel
            </button>
            <button
              @click="confirmDelete"
              :disabled="modalSaving"
              class="flex-1 inline-flex items-center justify-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50 transition-colors"
            >
              <svg v-if="modalSaving" class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
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
              {{ modalSaving ? 'Deleting…' : 'Delete User' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
