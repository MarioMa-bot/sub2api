<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 页头 + 标签页：规则配置 / 脱敏日志 -->
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.privacyMasking.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.privacyMasking.description') }}
          </p>
        </div>
        <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            type="button"
            role="tab"
            :aria-selected="activeTab === tab.value"
            class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors"
            :class="
              activeTab === tab.value
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
            "
            @click="switchTab(tab.value)"
          >
            {{ tab.label }}
          </button>
        </div>
      </div>

      <!-- ==================== 规则配置 ==================== -->
      <div v-if="activeTab === 'rules'" class="space-y-6">
        <div v-if="configLoading" class="flex items-center justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
        </div>

        <template v-else>
          <!-- 全局开关 -->
          <div class="card space-y-4 p-6">
            <div class="flex items-center justify-between gap-4">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.privacyMasking.enabled.title') }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyMasking.enabled.hint') }}
                </p>
              </div>
              <label class="relative inline-flex cursor-pointer items-center">
                <input v-model="config.enabled" type="checkbox" class="peer sr-only" />
                <div
                  class="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 dark:bg-gray-700 dark:peer-focus:ring-primary-800"
                ></div>
              </label>
            </div>
          </div>

          <!-- 基础配置 -->
          <div class="card space-y-4 p-6">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyMasking.basic.title') }}
            </h2>
            <div class="space-y-3">
              <label class="flex items-start gap-3">
                <input
                  v-model="config.mask_secrets"
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
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
              <label class="flex items-start gap-3">
                <input
                  v-model="config.mask_personal"
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
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

          <!-- 每种类型的检测开关与动作 -->
          <div class="card space-y-4 p-6">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.privacyMasking.actions.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyMasking.actions.description') }}
              </p>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div
                v-for="kind in builtinKinds"
                :key="kind"
                class="flex flex-wrap items-center justify-between gap-3 py-3"
              >
                <label class="flex items-start gap-3">
                  <input
                    type="checkbox"
                    class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="config.masked_kinds[kind] !== false"
                    @change="toggleKind(kind, ($event.target as HTMLInputElement).checked)"
                  />
                  <div>
                    <div class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ kindLabel(kind) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t(`admin.privacyMasking.actions.kinds.${kind}.description`) }}
                    </div>
                  </div>
                </label>
                <div class="w-40">
                  <Select v-model="config.kind_actions[kind]" :options="actionOptions" />
                </div>
              </div>
            </div>
          </div>

          <!-- 白名单 -->
          <div class="card space-y-4 p-6">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.privacyMasking.allowList.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyMasking.allowList.description') }}
              </p>
            </div>
            <textarea
              v-model="allowListText"
              class="input font-mono text-sm"
              rows="5"
              :placeholder="t('admin.privacyMasking.allowList.placeholder')"
            ></textarea>
          </div>

          <!-- 自定义规则 -->
          <div class="card space-y-4 p-6">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.privacyMasking.customRules.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyMasking.customRules.description') }}
              </p>
            </div>
            <div class="space-y-3">
              <div
                v-for="(rule, index) in config.custom_rules"
                :key="index"
                class="flex flex-wrap items-center gap-3"
              >
                <input
                  v-model.trim="rule.pattern"
                  type="text"
                  class="input min-w-[220px] flex-1 font-mono text-sm"
                  :placeholder="t('admin.privacyMasking.customRules.patternPlaceholder')"
                />
                <input
                  v-model.trim="rule.kind"
                  type="text"
                  class="input w-40 font-mono text-sm uppercase"
                  :placeholder="t('admin.privacyMasking.customRules.kindPlaceholder')"
                />
                <div class="w-40">
                  <Select v-model="rule.action" :options="actionOptions" />
                </div>
                <button type="button" class="btn btn-secondary text-red-600" @click="removeCustomRule(index)">
                  {{ t('admin.privacyMasking.customRules.remove') }}
                </button>
              </div>
              <button type="button" class="btn btn-secondary" @click="addCustomRule">
                {{ t('admin.privacyMasking.customRules.add') }}
              </button>
            </div>
          </div>

          <!-- 智能检测（Presidio 旁路，灰度功能） -->
          <div class="card space-y-4 border border-dashed border-gray-300 bg-gray-50/60 p-6 dark:border-dark-600 dark:bg-dark-800/40">
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="flex items-center gap-2">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.privacyMasking.smart.title') }}
                  </h2>
                  <span class="rounded-full bg-gray-200 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
                    {{ t('admin.privacyMasking.smart.badge') }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyMasking.smart.description') }}
                </p>
              </div>
              <label class="relative inline-flex shrink-0 cursor-pointer items-center">
                <input v-model="config.smart_masking_enabled" type="checkbox" class="peer sr-only" />
                <div
                  class="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 dark:bg-gray-700 dark:peer-focus:ring-primary-800"
                ></div>
              </label>
            </div>

            <div class="grid gap-4 md:grid-cols-3">
              <div class="md:col-span-3">
                <label class="input-label">{{ t('admin.privacyMasking.smart.url') }}</label>
                <div class="flex flex-wrap gap-3">
                  <input
                    v-model.trim="config.smart_sidecar_url"
                    type="text"
                    class="input min-w-[260px] flex-1 font-mono text-sm"
                    placeholder="http://privacy-ner:5002"
                  />
                  <button type="button" class="btn btn-secondary" :disabled="smartChecking" @click="checkSmartSidecar">
                    {{ smartChecking ? t('admin.privacyMasking.smart.checking') : t('admin.privacyMasking.smart.check') }}
                  </button>
                </div>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.privacyMasking.smart.urlHint') }}</p>
                <p
                  v-if="smartCheckResult"
                  class="mt-1 text-xs"
                  :class="smartCheckResult.ok ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
                >
                  {{
                    smartCheckResult.ok
                      ? t('admin.privacyMasking.smart.checkOk', { url: smartCheckResult.url, ms: smartCheckResult.latency_ms })
                      : t('admin.privacyMasking.smart.checkFailed', { error: smartCheckResult.error || '' })
                  }}
                </p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.privacyMasking.smart.timeout') }}</label>
                <input v-model.number="config.smart_timeout_ms" type="number" min="10" max="5000" step="10" class="input" />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.privacyMasking.smart.timeoutHint') }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.privacyMasking.smart.minScore') }}</label>
                <input v-model.number="config.smart_min_score" type="number" min="0" max="1" step="0.05" class="input" />
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.privacyMasking.smart.minScoreHint') }}</p>
              </div>
            </div>

            <div>
              <div class="input-label">{{ t('admin.privacyMasking.smart.entities') }}</div>
              <div class="divide-y divide-gray-100 dark:divide-dark-700">
                <div
                  v-for="kind in smartKinds"
                  :key="kind"
                  class="flex flex-wrap items-center justify-between gap-3 py-3"
                >
                  <label class="flex items-start gap-3">
                    <input
                      type="checkbox"
                      class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      :checked="config.smart_entities.includes(kind)"
                      @change="toggleSmartEntity(kind, ($event.target as HTMLInputElement).checked)"
                    />
                    <div>
                      <div class="text-sm font-medium text-gray-900 dark:text-white">{{ kindLabel(kind) }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t(`admin.privacyMasking.actions.kinds.${kind}.description`) }}
                      </div>
                    </div>
                  </label>
                  <div class="w-40">
                    <Select v-model="config.kind_actions[kind]" :options="actionOptions" />
                  </div>
                </div>
              </div>
            </div>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.privacyMasking.smart.fallbackHint') }}</p>
          </div>

          <div class="flex justify-end gap-3">
            <button type="button" class="btn btn-secondary" :disabled="saving" @click="loadConfig">
              {{ t('common.reset') }}
            </button>
            <button type="button" class="btn btn-primary" :disabled="saving" @click="saveConfig">
              {{ saving ? t('admin.privacyMasking.buttons.saving') : t('admin.privacyMasking.buttons.save') }}
            </button>
          </div>
        </template>
      </div>

      <!-- ==================== 脱敏日志 ==================== -->
      <div v-else class="space-y-6">
        <div
          v-if="!configLoading && !config.enabled"
          class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-300"
        >
          {{ t('admin.privacyMasking.logs.disabledHint') }}
        </div>

        <!-- 统计概览（最近 24 小时） -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div v-for="card in statCards" :key="card.key" class="card p-4">
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</div>
            <div class="mt-1 text-2xl font-semibold" :class="card.class">{{ card.value }}</div>
          </div>
        </div>

        <!-- 按 API 接口聚合：点击行即按该接口过滤 -->
        <div class="card p-4 sm:p-6">
          <div class="mb-3 flex items-center justify-between">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyMasking.logs.endpoints.title') }}
            </h2>
            <span class="text-xs text-gray-400">{{ t('admin.privacyMasking.logs.endpoints.hint') }}</span>
          </div>
          <div v-if="endpoints.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.privacyMasking.logs.empty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="min-w-full text-sm">
              <thead>
                <tr class="text-left text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  <th class="py-2 pr-4 font-medium">{{ t('admin.privacyMasking.logs.columns.endpoint') }}</th>
                  <th class="py-2 pr-4 text-right font-medium">{{ t('admin.privacyMasking.logs.endpoints.events') }}</th>
                  <th class="py-2 pr-4 text-right font-medium">{{ t('admin.privacyMasking.logs.endpoints.blocked') }}</th>
                  <th class="py-2 pr-4 text-right font-medium">{{ t('admin.privacyMasking.logs.endpoints.detected') }}</th>
                  <th class="py-2 font-medium">{{ t('admin.privacyMasking.logs.endpoints.lastSeen') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr
                  v-for="item in endpoints"
                  :key="item.endpoint"
                  class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-800"
                  :class="filters.endpoint === item.endpoint ? 'bg-primary-50 dark:bg-primary-900/20' : ''"
                  @click="selectEndpoint(item.endpoint)"
                >
                  <td class="py-2 pr-4 font-mono text-gray-800 dark:text-gray-200">
                    {{ item.endpoint || t('admin.privacyMasking.logs.unknownEndpoint') }}
                  </td>
                  <td class="py-2 pr-4 text-right text-gray-900 dark:text-white">{{ item.event_count }}</td>
                  <td class="py-2 pr-4 text-right" :class="item.blocked_count > 0 ? 'text-red-600' : 'text-gray-400'">
                    {{ item.blocked_count }}
                  </td>
                  <td class="py-2 pr-4 text-right text-gray-600 dark:text-gray-300">{{ item.detected_count }}</td>
                  <td class="whitespace-nowrap py-2 text-gray-500 dark:text-gray-400">{{ formatTime(item.last_seen_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <TablePageLayout>
          <!-- 过滤条件 -->
          <template #filters>
            <div class="card p-4 sm:p-6">
              <div class="flex flex-wrap items-end justify-between gap-4">
                <div class="flex flex-1 flex-wrap items-end gap-4">
                  <div class="w-full sm:w-auto sm:min-w-[240px]">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.endpoint') }}</label>
                    <Select v-model="filters.endpoint" :options="endpointOptions" searchable @change="search" />
                  </div>
                  <div class="w-full sm:w-auto sm:min-w-[150px]">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.kind') }}</label>
                    <Select v-model="filters.kind" :options="kindOptions" @change="search" />
                  </div>
                  <div class="w-full sm:w-auto sm:min-w-[130px]">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.result') }}</label>
                    <Select v-model="filters.blocked" :options="blockedOptions" @change="search" />
                  </div>
                  <div class="w-full sm:w-auto sm:min-w-[170px]">
                    <label class="input-label">{{ t('admin.dashboard.timeRange') }}</label>
                    <Select
                      :model-value="timeRange"
                      :options="timeRangeOptions"
                      @update:model-value="handleTimeRangeChange"
                    />
                  </div>
                  <div class="w-full sm:w-auto sm:min-w-[160px]">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.model') }}</label>
                    <input v-model.trim="filters.model" type="text" class="input" @keyup.enter="search" />
                  </div>
                  <div class="w-full sm:w-28">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.userId') }}</label>
                    <input v-model.trim="filters.user_id" type="text" inputmode="numeric" class="input" @keyup.enter="search" />
                  </div>
                  <div class="w-full sm:w-28">
                    <label class="input-label">{{ t('admin.privacyMasking.logs.filters.apiKeyId') }}</label>
                    <input v-model.trim="filters.api_key_id" type="text" inputmode="numeric" class="input" @keyup.enter="search" />
                  </div>
                </div>
                <div class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
                  <button type="button" class="btn btn-primary" :disabled="logsLoading" @click="search">
                    {{ t('common.search') }}
                  </button>
                  <button type="button" class="btn btn-secondary" :disabled="logsLoading" @click="resetFilters">
                    {{ t('common.reset') }}
                  </button>
                  <button type="button" class="btn btn-secondary" :disabled="logsLoading" @click="refreshAll">
                    <Icon name="refresh" size="sm" class="mr-1.5" />
                    {{ t('admin.privacyMasking.logs.refresh') }}
                  </button>
                </div>
              </div>
            </div>
          </template>

          <!-- 日志表格 -->
          <template #table>
            <DataTable :columns="columns" :data="logs" :loading="logsLoading" row-key="id">
              <template #cell-detected_at="{ value }">
                <span class="whitespace-nowrap text-gray-600 dark:text-gray-300">{{ formatTime(value) }}</span>
              </template>

              <template #cell-endpoint="{ row }">
                <div class="min-w-0 max-w-xs">
                  <button
                    type="button"
                    class="block max-w-full truncate font-mono text-sm text-gray-800 hover:text-primary-600 dark:text-gray-200"
                    :title="row.endpoint"
                    @click="selectEndpoint(row.endpoint || '')"
                  >
                    {{ row.endpoint || t('admin.privacyMasking.logs.unknownEndpoint') }}
                  </button>
                  <div class="mt-0.5 truncate text-xs text-gray-400" :title="row.model">{{ row.model || '—' }}</div>
                </div>
              </template>

              <template #cell-caller="{ row }">
                <div class="whitespace-nowrap text-xs text-gray-600 dark:text-gray-300">
                  <div>{{ t('admin.privacyMasking.logs.caller.user') }} #{{ row.user_id || '—' }}</div>
                  <div class="text-gray-400">
                    {{ t('admin.privacyMasking.logs.caller.apiKey') }} #{{ row.api_key_id ?? '—' }}
                    <span v-if="row.group_id"> · {{ t('admin.privacyMasking.logs.caller.group') }} #{{ row.group_id }}</span>
                  </div>
                </div>
              </template>

              <template #cell-kinds="{ row }">
                <div class="flex max-w-xs flex-wrap gap-1">
                  <span
                    v-for="(count, kind) in row.kinds"
                    :key="kind"
                    class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="actionBadgeClass(row.actions?.[kind])"
                    :title="actionLabel(row.actions?.[kind])"
                  >
                    {{ kindLabel(String(kind)) }}<span class="opacity-70">×{{ count }}</span>
                  </span>
                </div>
              </template>

              <template #cell-counts="{ row }">
                <div class="whitespace-nowrap text-xs">
                  <span class="text-green-700 dark:text-green-400">{{ t('admin.privacyMasking.logs.actions.masked') }} {{ row.masked_count }}</span>
                  <span class="mx-1 text-gray-300">/</span>
                  <span class="text-amber-700 dark:text-amber-400">{{ t('admin.privacyMasking.logs.actions.warned') }} {{ row.warned_count }}</span>
                  <span class="mx-1 text-gray-300">/</span>
                  <span class="text-red-700 dark:text-red-400">{{ t('admin.privacyMasking.logs.actions.blocked') }} {{ row.blocked_count }}</span>
                </div>
              </template>

              <template #cell-request_blocked="{ value }">
                <span
                  class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                  :class="
                    value
                      ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300'
                      : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
                  "
                >
                  {{ value ? t('admin.privacyMasking.logs.status.blocked') : t('admin.privacyMasking.logs.status.passed') }}
                </span>
              </template>

              <template #cell-request_id="{ value }">
                <span class="block max-w-[160px] truncate font-mono text-xs text-gray-400" :title="value">{{ value || '—' }}</span>
              </template>

              <template #empty>
                <div class="flex flex-col items-center py-8">
                  <Icon name="shield" size="xl" class="mb-4 h-12 w-12 text-gray-300 dark:text-dark-600" />
                  <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('admin.privacyMasking.logs.empty') }}</p>
                </div>
              </template>
            </DataTable>
          </template>

          <template #pagination>
            <Pagination
              v-if="total > 0"
              :total="total"
              :page="page"
              :page-size="pageSize"
              @update:page="onPageChange"
              @update:pageSize="onPageSizeChange"
            />
          </template>
        </TablePageLayout>
      </div>
    </div>

    <!-- 自定义时间范围 -->
    <BaseDialog
      :show="showCustomTimeRangeDialog"
      :title="t('admin.ops.timeRange.custom')"
      width="narrow"
      @close="showCustomTimeRangeDialog = false"
    >
      <div class="space-y-4 py-2">
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.startTime') }}</label>
          <input v-model="customStartTimeInput" type="datetime-local" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.endTime') }}</label>
          <input v-model="customEndTimeInput" type="datetime-local" class="input" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showCustomTimeRangeDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!customStartTimeInput || !customEndTimeInput"
          @click="handleCustomTimeRangeConfirm"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  adminAPI,
  type PrivacyMaskingAction,
  type PrivacyMaskingConfig,
  type PrivacyMaskingEndpointSummary,
  type PrivacyMaskingLog,
  type PrivacyMaskingSmartCheckResult,
  type PrivacyMaskingStats
} from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'

const { t, te } = useI18n()
const appStore = useAppStore()
const route = useRoute()
const router = useRouter()

type TabValue = 'rules' | 'logs'

// 内置规则支持的类型（ID_NUMBER 暂无内置规则，CUSTOM 由自定义规则产生）
const builtinKinds = ['API_KEY', 'PRIVATE_KEY', 'TOKEN', 'PASSWORD', 'EMAIL', 'PHONE', 'CREDIT_CARD']
// 智能检测（Presidio 旁路）可识别的实体类型
const smartKinds = ['PERSON', 'LOCATION', 'ORG', 'NRP', 'DATE_TIME']
const DEFAULT_SMART_ENTITIES = ['PERSON', 'LOCATION', 'ORG']

const tabs = computed<{ value: TabValue; label: string }[]>(() => [
  { value: 'rules', label: t('admin.privacyMasking.tabs.rules') },
  { value: 'logs', label: t('admin.privacyMasking.tabs.logs') }
])
const activeTab = ref<TabValue>(route.query.tab === 'logs' ? 'logs' : 'rules')

function switchTab(tab: TabValue) {
  activeTab.value = tab
  router.replace({ query: { ...route.query, tab: tab === 'rules' ? undefined : tab } })
  if (tab === 'logs') refreshAll()
}

// ==================== 规则配置 ====================

const configLoading = ref(true)
const saving = ref(false)
const config = ref<PrivacyMaskingConfig>(emptyConfig())
const allowListText = ref('')

function emptyConfig(): PrivacyMaskingConfig {
  return {
    enabled: false,
    mask_secrets: true,
    mask_personal: false,
    masked_kinds: {},
    custom_rules: [],
    allow_list: [],
    kind_actions: {},
    smart_masking_enabled: false,
    smart_sidecar_url: '',
    smart_timeout_ms: 80,
    smart_min_score: 0.6,
    smart_entities: [...DEFAULT_SMART_ENTITIES]
  }
}

const actionOptions = computed(() => [
  { value: 'mask', label: t('admin.privacyMasking.actions.mask') },
  { value: 'warn', label: t('admin.privacyMasking.actions.warn') },
  { value: 'block', label: t('admin.privacyMasking.actions.block') }
])

function normalizeConfig(raw: PrivacyMaskingConfig): PrivacyMaskingConfig {
  const cfg = { ...emptyConfig(), ...raw }
  cfg.masked_kinds = { ...(raw.masked_kinds || {}) }
  cfg.kind_actions = { ...(raw.kind_actions || {}) }
  cfg.custom_rules = [...(raw.custom_rules || [])]
  cfg.allow_list = [...(raw.allow_list || [])]
  cfg.smart_entities = [...(raw.smart_entities ?? DEFAULT_SMART_ENTITIES)]
  // 未配置的类型默认“脱敏”，让下拉框有明确取值
  for (const kind of [...builtinKinds, ...smartKinds]) {
    if (!cfg.kind_actions[kind]) cfg.kind_actions[kind] = 'mask'
  }
  return cfg
}

async function loadConfig() {
  configLoading.value = true
  try {
    config.value = normalizeConfig(await adminAPI.privacyMasking.getConfig())
    allowListText.value = config.value.allow_list.join('\n')
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.privacyMasking.messages.loadError'))
  } finally {
    configLoading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  try {
    const payload: PrivacyMaskingConfig = {
      ...config.value,
      allow_list: allowListText.value
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line.length > 0),
      custom_rules: config.value.custom_rules
        .filter((rule) => rule.pattern)
        .map((rule) => ({ ...rule, kind: (rule.kind || 'CUSTOM').toUpperCase() }))
    }
    config.value = normalizeConfig(await adminAPI.privacyMasking.updateConfig(payload))
    allowListText.value = config.value.allow_list.join('\n')
    appStore.showSuccess(t('admin.privacyMasking.messages.saveSuccess'))
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.privacyMasking.messages.saveError'))
  } finally {
    saving.value = false
  }
}

function toggleKind(kind: string, enabled: boolean) {
  if (enabled) {
    delete config.value.masked_kinds[kind]
  } else {
    config.value.masked_kinds[kind] = false
  }
}

function toggleSmartEntity(kind: string, enabled: boolean) {
  const set = new Set(config.value.smart_entities)
  if (enabled) set.add(kind)
  else set.delete(kind)
  config.value.smart_entities = smartKinds.filter((k) => set.has(k))
}

const smartChecking = ref(false)
const smartCheckResult = ref<PrivacyMaskingSmartCheckResult | null>(null)

async function checkSmartSidecar() {
  smartChecking.value = true
  try {
    smartCheckResult.value = await adminAPI.privacyMasking.checkSmartSidecar(config.value.smart_sidecar_url)
  } catch (err: any) {
    smartCheckResult.value = { ok: false, url: config.value.smart_sidecar_url, latency_ms: 0, error: err?.message || String(err) }
  } finally {
    smartChecking.value = false
  }
}

function addCustomRule() {
  config.value.custom_rules.push({ pattern: '', kind: 'CUSTOM', action: 'mask' })
}

function removeCustomRule(index: number) {
  config.value.custom_rules.splice(index, 1)
}

// ==================== 脱敏日志 ====================

const logsLoading = ref(false)
const logs = ref<PrivacyMaskingLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const endpoints = ref<PrivacyMaskingEndpointSummary[]>([])
const stats = ref<PrivacyMaskingStats | null>(null)

const filters = reactive({
  endpoint: '',
  kind: '',
  blocked: '',
  model: '',
  user_id: '',
  api_key_id: ''
})

const timeRange = ref('24h')
const customStartTime = ref('')
const customEndTime = ref('')
const showCustomTimeRangeDialog = ref(false)
const customStartTimeInput = ref('')
const customEndTimeInput = ref('')

const TIME_RANGE_MINUTES: Record<string, number> = {
  '30m': 30,
  '1h': 60,
  '6h': 6 * 60,
  '24h': 24 * 60,
  '7d': 7 * 24 * 60,
  '30d': 30 * 24 * 60
}

const timeRangeOptions = computed(() => [
  { value: '', label: t('admin.privacyMasking.logs.filters.all') },
  ...Object.keys(TIME_RANGE_MINUTES).map((value) => ({ value, label: t(`admin.ops.timeRange.${value}`) })),
  { value: 'custom', label: t('admin.ops.timeRange.custom') }
])

const endpointOptions = computed(() => {
  const options = [{ value: '', label: t('admin.privacyMasking.logs.filters.allEndpoints') }]
  const seen = new Set<string>()
  for (const item of endpoints.value) {
    if (!item.endpoint || seen.has(item.endpoint)) continue
    seen.add(item.endpoint)
    options.push({ value: item.endpoint, label: item.endpoint })
  }
  // 当前过滤的接口不在聚合结果里（例如切换了时间范围）时仍保留选项
  if (filters.endpoint && !seen.has(filters.endpoint)) {
    options.push({ value: filters.endpoint, label: filters.endpoint })
  }
  return options
})

const kindOptions = computed(() => [
  { value: '', label: t('admin.privacyMasking.logs.filters.all') },
  ...[...builtinKinds, ...smartKinds, 'CUSTOM'].map((kind) => ({ value: kind, label: kindLabel(kind) }))
])

const blockedOptions = computed(() => [
  { value: '', label: t('admin.privacyMasking.logs.filters.all') },
  { value: 'true', label: t('admin.privacyMasking.logs.status.blocked') },
  { value: 'false', label: t('admin.privacyMasking.logs.status.passed') }
])

const columns = computed<Column[]>(() => [
  { key: 'detected_at', label: t('admin.privacyMasking.logs.columns.time') },
  { key: 'endpoint', label: t('admin.privacyMasking.logs.columns.endpoint') },
  { key: 'caller', label: t('admin.privacyMasking.logs.columns.caller') },
  { key: 'kinds', label: t('admin.privacyMasking.logs.columns.kinds') },
  { key: 'counts', label: t('admin.privacyMasking.logs.columns.action') },
  { key: 'request_blocked', label: t('admin.privacyMasking.logs.columns.status') },
  { key: 'request_id', label: t('admin.privacyMasking.logs.columns.requestId') }
])

const statCards = computed(() => [
  {
    key: 'events',
    label: t('admin.privacyMasking.logs.stats.events'),
    value: stats.value?.event_count ?? 0,
    class: 'text-gray-900 dark:text-white'
  },
  {
    key: 'blocked',
    label: t('admin.privacyMasking.logs.stats.blocked'),
    value: stats.value?.blocked_count ?? 0,
    class: 'text-red-600'
  },
  {
    key: 'masked',
    label: t('admin.privacyMasking.logs.stats.masked'),
    value: stats.value?.masked_count ?? 0,
    class: 'text-green-600'
  },
  {
    key: 'warned',
    label: t('admin.privacyMasking.logs.stats.warned'),
    value: stats.value?.warned_count ?? 0,
    class: 'text-amber-600'
  }
])

function kindLabel(kind: string): string {
  const key = `admin.privacyMasking.actions.kinds.${kind}.label`
  return te(key) ? t(key) : kind
}

function actionLabel(action?: PrivacyMaskingAction): string {
  return action ? t(`admin.privacyMasking.actions.${action}`) : ''
}

function actionBadgeClass(action?: PrivacyMaskingAction): string {
  switch (action) {
    case 'block':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300'
    case 'warn':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300'
    default:
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300'
  }
}

function toDatetimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function toRFC3339(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}

function buildTimeRangeQuery(): { start_time?: string; end_time?: string } {
  if (timeRange.value === 'custom') {
    return { start_time: toRFC3339(customStartTime.value), end_time: toRFC3339(customEndTime.value) }
  }
  const minutes = TIME_RANGE_MINUTES[timeRange.value]
  if (!minutes) return {}
  return { start_time: new Date(Date.now() - minutes * 60 * 1000).toISOString() }
}

function positiveInt(value: string): number | undefined {
  const n = Number(value)
  return Number.isInteger(n) && n > 0 ? n : undefined
}

function handleTimeRangeChange(val: string | number | boolean | null) {
  const value = String(val ?? '')
  if (value === 'custom') {
    const now = new Date()
    customStartTimeInput.value = customStartTime.value || toDatetimeLocal(new Date(now.getTime() - 60 * 60 * 1000))
    customEndTimeInput.value = customEndTime.value || toDatetimeLocal(now)
    showCustomTimeRangeDialog.value = true
    return
  }
  timeRange.value = value
  refreshAll()
}

function handleCustomTimeRangeConfirm() {
  customStartTime.value = customStartTimeInput.value
  customEndTime.value = customEndTimeInput.value
  timeRange.value = 'custom'
  showCustomTimeRangeDialog.value = false
  refreshAll()
}

async function fetchLogs() {
  logsLoading.value = true
  try {
    const res = await adminAPI.privacyMasking.listLogs({
      page: page.value,
      page_size: pageSize.value,
      endpoint: filters.endpoint || undefined,
      kind: filters.kind || undefined,
      blocked: filters.blocked || undefined,
      model: filters.model || undefined,
      user_id: positiveInt(filters.user_id),
      api_key_id: positiveInt(filters.api_key_id),
      ...buildTimeRangeQuery()
    })
    logs.value = res.items || []
    total.value = res.total || 0
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.privacyMasking.messages.loadLogsError'))
  } finally {
    logsLoading.value = false
  }
}

async function fetchSummary() {
  try {
    const range = buildTimeRangeQuery()
    const [endpointItems, statsData] = await Promise.all([
      adminAPI.privacyMasking.listEndpoints(range),
      adminAPI.privacyMasking.getStats(range.start_time ?? new Date(0).toISOString())
    ])
    endpoints.value = endpointItems
    stats.value = statsData
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.privacyMasking.messages.loadLogsError'))
  }
}

function search() {
  page.value = 1
  fetchLogs()
}

function refreshAll() {
  fetchSummary()
  search()
}

function selectEndpoint(endpoint: string) {
  filters.endpoint = filters.endpoint === endpoint ? '' : endpoint
  search()
}

function resetFilters() {
  Object.assign(filters, { endpoint: '', kind: '', blocked: '', model: '', user_id: '', api_key_id: '' })
  timeRange.value = '24h'
  refreshAll()
}

function onPageChange(p: number) {
  page.value = p
  fetchLogs()
}

function onPageSizeChange(size: number) {
  pageSize.value = size
  search()
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

onMounted(async () => {
  await loadConfig()
  if (activeTab.value === 'logs') refreshAll()
})
</script>
