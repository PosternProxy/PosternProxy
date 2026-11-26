<script setup lang="ts">
import lightLogo from '../../../internal/branding/logo.png'
import darkLogo from '../../../internal/branding/logo_dark.png'

defineProps<{ dark?: boolean }>()
</script>

<template>
  <img :src="dark ? darkLogo : lightLogo" alt="PosternProxy" width="320" height="320" />
</template>
