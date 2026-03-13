<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { authState, checkAuthStatus, login } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';
import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

const router = useRouter();
const password = ref('');
const loading = ref(false);
const errorMessage = ref('');
const { t } = useLocale();

const submit = async () => {
  loading.value = true;
  errorMessage.value = '';
  try {
    await login(password.value);
    router.push('/');
  } catch {
    errorMessage.value = t('invalidPassword');
  } finally {
    loading.value = false;
  }
};

onMounted(async () => {
  const status = await checkAuthStatus();
  if (!status.authEnabled || status.authenticated) {
    router.push('/');
  }
});
</script>

<template>
  <div class="app-page login-shell">
    <Card class="glass-panel login-card p-6">
      <h1 class="text-2xl font-extrabold tracking-tight">{{ t('signIn') }}</h1>
      <p class="mt-1 text-sm text-[var(--text-2)]">{{ t('signInSubtitle') }}</p>

      <Alert v-if="errorMessage" variant="error" class="mt-4">
        {{ errorMessage }}
      </Alert>

      <div class="mt-4 grid gap-3">
        <label class="text-sm font-semibold text-[var(--text-2)]">{{ t('password') }}</label>
        <Input
          v-model="password"
          type="password"
          :placeholder="t('password')"
          @keyup.enter="submit"
        />
      </div>

      <Button
        class="mt-5 w-full"
        :disabled="loading || authState.authLoading.value"
        @click="submit"
      >
        {{ t('continue') }}
      </Button>
    </Card>
  </div>
</template>

<style scoped>
.login-shell {
  min-height: calc(100vh - 72px);
  display: grid;
  place-items: center;
  padding: 16px;
}

.login-card {
  width: min(430px, 100%);
}
</style>
