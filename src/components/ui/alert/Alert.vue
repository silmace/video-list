<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

type AlertVariant = 'default' | 'warning' | 'error' | 'success'

const props = withDefaults(defineProps<{ variant?: AlertVariant; class?: string }>(), {
  variant: 'default',
  class: '',
})

const classes = computed(() => {
  const variantClassMap: Record<AlertVariant, string> = {
    default: 'border-border bg-card text-card-foreground',
    warning: 'border-amber-600/40 bg-amber-500/10 text-amber-900 dark:text-amber-200',
    error: 'border-destructive/40 bg-destructive/10 text-destructive',
    success: 'border-emerald-600/40 bg-emerald-500/10 text-emerald-800 dark:text-emerald-200',
  }

  return cn('relative w-full rounded-lg border px-4 py-3 text-sm', variantClassMap[props.variant], props.class)
})
</script>

<template>
  <div :class="classes" role="alert">
    <slot />
  </div>
</template>
