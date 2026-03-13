<script setup lang="ts">
import { cn } from '@/lib/utils'

withDefaults(defineProps<{ class?: string }>(), {
  class: '',
})
</script>

<template>
  <div :class="cn('rounded-xl border bg-card text-card-foreground shadow-sm', $props.class)">
    <slot />
  </div>
</template>
