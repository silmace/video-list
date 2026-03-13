<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { authState, checkAuthStatus, login } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';

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
  <v-container class="fill-height d-flex align-center justify-center">
    <v-card class="glass-panel pa-6" max-width="420" width="100%">
      <v-card-title class="text-h5">{{ t('signIn') }}</v-card-title>
      <v-card-subtitle class="mb-4">{{ t('signInSubtitle') }}</v-card-subtitle>

      <v-alert
        v-if="errorMessage"
        type="error"
        variant="tonal"
        density="comfortable"
        class="mb-4"
      >
        {{ errorMessage }}
      </v-alert>

      <v-text-field
        v-model="password"
        type="password"
        :label="t('password')"
        variant="outlined"
        density="comfortable"
        @keyup.enter="submit"
      />

      <v-btn
        color="primary"
        block
        size="large"
        :loading="loading || authState.authLoading.value"
        @click="submit"
      >
        {{ t('continue') }}
      </v-btn>
    </v-card>
  </v-container>
</template>
