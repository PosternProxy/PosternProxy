import { computed, ref, type ComputedRef, type Ref } from 'vue'

// Resource items must carry a numeric id. Every list view's row type
// satisfies this — it comes from the database PK.
export interface HasID {
  id: number
}

export interface ResourceListOptions<T extends HasID> {
  /** Loads the full collection. Called by load(). */
  list: () => Promise<T[]>
  /** Deletes one item by id. Called by doDelete() after delete-confirm. */
  delete: (id: number) => Promise<void>
  /**
   * Human noun used in the default failure messages, e.g. 'proxy host'.
   * Pluralized automatically by adding 's' for load errors.
   */
  itemNoun?: string
}

export interface ResourceListReturn<T extends HasID> {
  // ── Collection ─────────────────────────────────────────────────────
  items: Ref<T[]>
  loading: Ref<boolean>
  loadError: Ref<string>
  load: () => Promise<void>

  // ── Create/Edit modal state ───────────────────────────────────────
  showModal: Ref<boolean>
  editingId: Ref<number | null>
  modalMode: ComputedRef<'create' | 'edit'>
  saving: Ref<boolean>
  saveError: Ref<string>
  openCreate: () => void
  openEdit: (item: T) => void
  closeModal: () => void

  // ── Delete-confirm state (feeds <DeleteConfirmDialog>) ────────────
  showDeleteConfirm: Ref<boolean>
  deletingId: Ref<number | null>
  deleting: Ref<boolean>
  deleteError: Ref<string>
  confirmDelete: (id: number) => void
  doDelete: () => Promise<void>

  // ── Mutation helpers (used by view-local save handlers) ───────────
  /** Replace an item in items by id, or append if not present. */
  upsert: (item: T) => void
  /** Append a new item. Use after a successful create. */
  appendItem: (item: T) => void
}

/**
 * useResourceList encapsulates the modal + delete-confirm + load state that
 * every list view in this app shares. It's the shape behind the L-2/L-8 fixes
 * (the doDelete body in particular): on failure, keep the modal open and the
 * row in the local list; surface the error.
 *
 * Resource-specific concerns — form schema, validation, the actual create/
 * update API calls — stay in the view. The view calls upsert() / appendItem()
 * after a successful save.
 */
export function useResourceList<T extends HasID>(
  options: ResourceListOptions<T>
): ResourceListReturn<T> {
  // Collection
  const items = ref([]) as Ref<T[]>
  const loading = ref(false)
  const loadError = ref('')

  // Create/Edit
  const showModal = ref(false)
  const editingId = ref<number | null>(null)
  const saving = ref(false)
  const saveError = ref('')
  const modalMode = computed<'create' | 'edit'>(() =>
    editingId.value === null ? 'create' : 'edit'
  )

  // Delete-confirm
  const showDeleteConfirm = ref(false)
  const deletingId = ref<number | null>(null)
  const deleting = ref(false)
  const deleteError = ref('')

  const noun = options.itemNoun ?? 'item'

  async function load() {
    loading.value = true
    loadError.value = ''
    try {
      items.value = await options.list()
    } catch (e: unknown) {
      loadError.value = e instanceof Error ? e.message : `Failed to load ${noun}s.`
    } finally {
      loading.value = false
    }
  }

  function openCreate() {
    editingId.value = null
    saveError.value = ''
    showModal.value = true
  }

  function openEdit(item: T) {
    editingId.value = item.id
    saveError.value = ''
    showModal.value = true
  }

  function closeModal() {
    showModal.value = false
    editingId.value = null
  }

  function confirmDelete(id: number) {
    deletingId.value = id
    deleteError.value = ''
    showDeleteConfirm.value = true
  }

  async function doDelete() {
    if (deletingId.value === null) return
    deleting.value = true
    deleteError.value = ''
    try {
      await options.delete(deletingId.value)
      items.value = items.value.filter((i) => i.id !== deletingId.value)
      showDeleteConfirm.value = false
    } catch (e: unknown) {
      deleteError.value = e instanceof Error ? e.message : `Failed to delete the ${noun}.`
    } finally {
      deleting.value = false
    }
  }

  function upsert(item: T) {
    const idx = items.value.findIndex((i) => i.id === item.id)
    if (idx >= 0) {
      items.value[idx] = item
    } else {
      items.value.push(item)
    }
  }

  function appendItem(item: T) {
    items.value.push(item)
  }

  return {
    items,
    loading,
    loadError,
    load,
    showModal,
    editingId,
    modalMode,
    saving,
    saveError,
    openCreate,
    openEdit,
    closeModal,
    showDeleteConfirm,
    deletingId,
    deleting,
    deleteError,
    confirmDelete,
    doDelete,
    upsert,
    appendItem,
  }
}
