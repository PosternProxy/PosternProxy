<script setup lang="ts">
// Shared delete-confirm dialog used by every list view. The behaviour and
// markup match the post-L-2/L-8 audit pattern: the modal stays open and
// shows the error string when `error` is non-empty (so the caller can keep
// the row in the local list); backdrop click and Cancel both close without
// emitting `confirm`.
interface Props {
  modelValue: boolean
  title: string
  message: string
  confirmLabel?: string
  busyLabel?: string
  error?: string
  busy?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  confirmLabel: 'Delete',
  busyLabel: undefined,
  error: '',
  busy: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: []
}>()

function close() {
  emit('update:modelValue', false)
}

function busyText() {
  return props.busyLabel ?? `${props.confirmLabel}…`
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="close"
    >
      <div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-2xl">
        <h3 class="mb-2 text-base font-semibold text-slate-800">{{ title }}</h3>
        <p class="text-sm text-slate-600">{{ message }}</p>
        <div v-if="error" class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
          {{ error }}
        </div>
        <div class="mt-5 flex justify-end gap-3">
          <button
            class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
            :disabled="busy"
            @click="close"
          >
            Cancel
          </button>
          <button
            class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-500 disabled:opacity-50"
            :disabled="busy"
            @click="emit('confirm')"
          >
            {{ busy ? busyText() : confirmLabel }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
