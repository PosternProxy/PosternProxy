import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useServerStore } from './servers'

export const useServerContext = defineStore('serverContext', () => {
  const serverStore = useServerStore()

  const _stored = localStorage.getItem('selectedServerId')
  const selectedServerId = ref<number>(_stored ? parseInt(_stored) : 1)

  watch(selectedServerId, (id) => {
    localStorage.setItem('selectedServerId', String(id))
  })

  const selectedServer = computed(
    () => serverStore.getById(selectedServerId.value) ?? serverStore.servers[0]
  )

  // Reset to 1 if the stored ID is no longer valid after servers load
  watch(
    () => serverStore.loaded,
    (loaded) => {
      if (loaded && !serverStore.getById(selectedServerId.value)) {
        selectedServerId.value = 1
      }
    }
  )

  function select(id: number) {
    selectedServerId.value = id
  }

  return { selectedServerId, selectedServer, select }
})
