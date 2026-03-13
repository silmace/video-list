<script setup lang="ts">
import { cn } from '@/lib/utils'

withDefaults(
  defineProps<{
    modelValue?: string | number
    type?: string
    placeholder?: string
    class?: string
    disabled?: boolean
  }>(),
  {
    modelValue: '',
    type: 'text',
    placeholder: '',
    class: '',
    disabled: false,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <input
    :value="modelValue"
    :type="type"
    :placeholder="placeholder"
    :disabled="disabled"
    :class="cn('flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50', $props.class)"
    @input="handleInput"
  >
</template>
