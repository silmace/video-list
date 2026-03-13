<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

type BadgeVariant = 'default' | 'secondary' | 'outline' | 'destructive' | 'success'

const props = withDefaults(defineProps<{ variant?: BadgeVariant; class?: string }>(), {
  variant: 'default',
  class: '',
})

const classes = computed(() => {
  const variantClassMap: Record<BadgeVariant, string> = {
    default: 'bg-primary/15 text-primary border-transparent',
    secondary: 'bg-secondary/20 text-secondary border-transparent',
    outline: 'bg-transparent text-foreground border-border',
    destructive: 'bg-destructive/20 text-destructive border-transparent',
    success: 'bg-emerald-600/20 text-emerald-700 dark:text-emerald-300 border-transparent',
  }

  return cn(
    'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold',
    variantClassMap[props.variant],
    props.class
  )
})
</script>

<template>
  <span :class="classes">
    <slot />
  </span>
</template>
