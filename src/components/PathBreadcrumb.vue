<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { useLocale } from '../composables/useLocale';

const props = defineProps<{
  path: string;
  onNavigate?: (path: string) => void;
}>();

const router = useRouter();
const { t } = useLocale();

const pathSegments = computed(() => {
  const segments = props.path.split('/').filter(Boolean);
  return [
    { name: t('home'), path: '/' },
    ...segments.map((segment, index) => ({
      name: segment,
      path: '/' + segments.slice(0, index + 1).join('/') + '/'
    }))
  ];
});

const handleClick = (path: string) => {
  if (props.onNavigate) {
    props.onNavigate(path);
  } else {
    router.push(path);
  }
};
</script>

<template>
  <nav class="crumb-root" :aria-label="t('path')">
    <span class="crumb-label">{{ t('path') }}</span>
    <template v-for="(segment, index) in pathSegments" :key="segment.path">
      <button type="button" class="crumb-btn" @click="handleClick(segment.path)">
        {{ segment.name }}
      </button>
      <span v-if="index < pathSegments.length - 1" class="crumb-sep">/</span>
    </template>
  </nav>
</template>

<style scoped>
.crumb-root {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.crumb-label {
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: color-mix(in srgb, var(--accent, #3b82f6) 45%, #64748b);
}

.crumb-btn {
  border: none;
  border-radius: 10px;
  padding: 4px 8px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  transition: background 0.2s ease;
}

.crumb-btn:hover {
  background: color-mix(in srgb, var(--accent, #3b82f6) 15%, transparent);
}

.crumb-sep {
  color: #94a3b8;
}
</style>