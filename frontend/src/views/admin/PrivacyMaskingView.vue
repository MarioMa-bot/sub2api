<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="flex items-center justify-between">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('admin.privacyMasking.title') }}
        </h1>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <!-- Configuration Form -->
      <div v-else class="space-y-6">
        <!-- Global Enable Switch -->
        <div class="card p-6 space-y-4">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.privacyMasking.enabled.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyMasking.enabled.description') }}
              </p>
            </div>
            <label class="relative inline-flex cursor-pointer items-center">
              <input
                type="checkbox"
                v-model="config.enabled"
                class="peer sr-only"
                @change="saveConfig"
              />
              <div class="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 dark:border-gray-600 dark:bg-gray-700 dark:peer-focus:ring-primary-800"></div>
            </label>
          </div>
        </div>

        <!-- Basic Settings -->
        <div class="card p-6 space-y-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.privacyMasking.basic.title') }}
          </h2>

          <div class="space-y-3">
            <label class="flex items-start space-x-3">
              <input
                type="checkbox"
                v-model="config.mask_secrets"
                class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                @change="saveConfig"
              />
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('admin.privacyMasking.basic.maskSecrets.title') }}
                </div>
                <div class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyMasking.basic.maskSecrets.description') }}
                </div>
              </div>
            </label>

            <label class="flex items-start space-x-3">
              <input
                type="checkbox"
                v-model="config.mask_personal"
                class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                @change="saveConfig"
              />
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('admin.privacyMasking.basic.maskPersonal.title') }}
                </div>
                <div class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyMasking.basic.maskPersonal.description') }}
                </div>
              </div>
            </label>
          </div>
        </div>

        <!-- Action Settings -->
        <div class="card p-6 space-y-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.privacyMasking.actions.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.privacyMasking.actions.description') }}
          </p>

          <div class="space-y-3">
            <div v-for="kind in sensitiveKinds" :key="kind.value" class="flex items-center justify-between">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t(`admin.privacyMasking.actions.kinds.${kind.value}.label`) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t(`admin.privacyMasking.actions.kinds.${kind.value}.description`) }}
                </div>
              </div>
              <select
                v-model="config.kind_actions[kind.value]"
                class="rounded-md border-gray-300 text-sm focus:border-primary-500 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                @change="saveConfig"
              >
                <option value="mask">{{ t('admin.privacyMasking.actions.mask') }}</option>
                <option value="warn">{{ t('admin.privacyMasking.actions.warn') }}</option>
                <option value="block">{{ t('admin.privacyMasking.actions.block') }}</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Allow List -->
        <div class="card p-6 space-y-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.privacyMasking.allowList.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.privacyMasking.allowList.description') }}
          </p>

          <textarea
            v-model="allowListText"
            class="w-full rounded-md border-gray-300 font-mono text-sm focus:border-primary-500 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            rows="5"
            :placeholder="t('admin.privacyMasking.allowList.placeholder')"
            @blur="updateAllowList"
          ></textarea>
        </div>

        <!-- Custom Rules -->
        <div class="card p-6 space-y-4">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.privacyMasking.customRules.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.privacyMasking.customRules.description') }}
          </p>

          <div class="space-y-3">
            <div
              v-for="(rule, index) in config.custom_rules"
              :key="index"
              class="flex items-start space-x-3"
            >
              <input
                v-model="rule.pattern"
                type="text"
                class="flex-1 rounded-md border-gray-300 text-sm font-mono focus:border-primary-500 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                :placeholder="t('admin.privacyMasking.customRules.patternPlaceholder')"
              />
              <select
                v-model="rule.action"
                class="rounded-md border-gray-300 text-sm focus:border-primary-500 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              >
                <option value="mask">{{ t('admin.privacyMasking.actions.mask') }}</option>
                <option value="warn">{{ t('admin.privacyMasking.actions.warn') }}</option>
                <option value="block">{{ t('admin.privacyMasking.actions.block') }}</option>
              </select>
              <button
                type="button"
                class="rounded-md p-2 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
                @click="removeCustomRule(index)"
              >
                {{ t('admin.privacyMasking.customRules.remove') }}
              </button>
            </div>

            <button
              type="button"
              class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
              @click="addCustomRule"
            >
              {{ t('admin.privacyMasking.customRules.add') }}
            </button>
          </div>

          <div class="mt-4 flex justify-end">
            <button
              type="button"
              class="rounded-md bg-primary-600 px-6 py-2 text-sm font-medium text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2"
              @click="saveConfig"
              :disabled="saving"
            >
              {{ saving ? t('admin.privacyMasking.buttons.saving') : t('admin.privacyMasking.buttons.save') }}
            </button>
          </div>
        </div>

        <!-- Logs Section -->
        <div class="card p-6 space-y-4">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyMasking.logs.title') }}
            </h2>
            <button
              type="button"
              class="text-sm text-primary-600 hover:text-primary-700"
              @click="loadLogs"
            >
              {{ t('admin.privacyMasking.logs.refresh') }}
            </button>
          </div>

          <div v-if="logsLoading" class="flex justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
          </div>

          <div v-else-if="logs.length === 0" class="py-8 text-center text-sm text-gray-500">
            {{ t('admin.privacyMasking.logs.empty') }}
          </div>

          <div v-else class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead>
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {{ t('admin.privacyMasking.logs.columns.time') }}
                  </th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {{ t('admin.privacyMasking.logs.columns.kinds') }}
                  </th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {{ t('admin.privacyMasking.logs.columns.count') }}
                  </th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {{ t('admin.privacyMasking.logs.columns.action') }}
                  </th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    {{ t('admin.privacyMasking.logs.columns.status') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <tr v-for="log in logs" :key="log.id">
                  <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-900 dark:text-white">
                    {{ formatDate(log.detected_at) }}
                  </td>
                  <td class="px-4 py-3 text-sm text-gray-900 dark:text-white">
                    {{ Object.keys(log.kinds).join(', ') }}
                  </td>
                  <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-900 dark:text-white">
                    {{ log.total_count }}
                  </td>
                  <td class="px-4 py-3 text-sm">
                    <span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium" :class="{
                      'bg-green-100 text-green-800': log.masked_count > 0,
                      'bg-yellow-100 text-yellow-800': log.warned_count > 0,
                      'bg-red-100 text-red-800': log.blocked_count > 0
                    }">
                      {{ getActionLabel(log) }}
                    </span>
                  </td>
                  <td class="whitespace-nowrap px-4 py-3 text-sm">
                    <span v-if="log.request_blocked" class="inline-flex items-center rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-800">
                      {{ t('admin.privacyMasking.logs.status.blocked') }}
                    </span>
                    <span v-else class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">
                      {{ t('admin.privacyMasking.logs.status.passed') }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/AppLayout.vue'
import { apiClient } from '@/lib/api-client'

const { t } = useI18n()

interface PrivacyMaskingConfig {
  id?: number
  enabled: boolean
  mask_secrets: boolean
  mask_personal: boolean
  masked_kinds: Record<string, boolean>
  custom_rules: Array<{
    pattern: string
    kind: string
    action: string
  }>
  allow_list: string[]
  kind_actions: Record<string, string>
}

interface PrivacyMaskingLog {
  id: number
  request_id: string
  user_id: number
  detected_at: string
  kinds: Record<string, number>
  actions: Record<string, string>
  total_count: number
  masked_count: number
  warned_count: number
  blocked_count: number
  request_blocked: boolean
}

const loading = ref(true)
const saving = ref(false)
const logsLoading = ref(false)
const logs = ref<PrivacyMaskingLog[]>([])

const config = ref<PrivacyMaskingConfig>({
  enabled: false,
  mask_secrets: true,
  mask_personal: false,
  masked_kinds: {},
  custom_rules: [],
  allow_list: [],
  kind_actions: {}
})

const allowListText = ref('')

const sensitiveKinds = [
  { value: 'API_KEY' },
  { value: 'PRIVATE_KEY' },
  { value: 'TOKEN' },
  { value: 'PASSWORD' },
  { value: 'EMAIL' },
  { value: 'PHONE' },
  { value: 'CREDIT_CARD' }
]

onMounted(async () => {
  await loadConfig()
  await loadLogs()
  loading.value = false
})

async function loadConfig() {
  try {
    const response = await apiClient.get('/api/admin/privacy-masking/config')
    config.value = response.data
    allowListText.value = config.value.allow_list?.join('\n') || ''
  } catch (error) {
    console.error('Failed to load config:', error)
  }
}

async function saveConfig() {
  saving.value = true
  try {
    const response = await apiClient.put('/api/admin/privacy-masking/config', config.value)
    config.value = response.data
  } catch (error) {
    console.error('Failed to save config:', error)
  } finally {
    saving.value = false
  }
}

async function loadLogs() {
  logsLoading.value = true
  try {
    const response = await apiClient.get('/api/admin/privacy-masking/logs?limit=50')
    logs.value = response.data.logs || []
  } catch (error) {
    console.error('Failed to load logs:', error)
  } finally {
    logsLoading.value = false
  }
}

function updateAllowList() {
  config.value.allow_list = allowListText.value
    .split('\n')
    .map(line => line.trim())
    .filter(line => line.length > 0)
  saveConfig()
}

function addCustomRule() {
  config.value.custom_rules.push({
    pattern: '',
    kind: 'CUSTOM',
    action: 'mask'
  })
}

function removeCustomRule(index: number) {
  config.value.custom_rules.splice(index, 1)
  saveConfig()
}

function formatDate(dateStr: string) {
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN')
}

function getActionLabel(log: PrivacyMaskingLog) {
  if (log.masked_count > 0) return t('admin.privacyMasking.logs.actions.masked')
  if (log.warned_count > 0) return t('admin.privacyMasking.logs.actions.warned')
  return t('admin.privacyMasking.logs.actions.blocked')
}
</script>
