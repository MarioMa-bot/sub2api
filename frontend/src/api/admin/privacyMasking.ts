/**
 * 隐私脱敏管理 API：规则配置、脱敏日志（仅元数据，不含原始敏感值）、按 API 接口统计。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type PrivacyMaskingAction = 'mask' | 'warn' | 'block'

export interface PrivacyMaskingRule {
  pattern: string
  kind: string
  action: PrivacyMaskingAction
}

export interface PrivacyMaskingConfig {
  id?: number
  enabled: boolean
  mask_secrets: boolean
  mask_personal: boolean
  masked_kinds: Record<string, boolean>
  custom_rules: PrivacyMaskingRule[]
  allow_list: string[]
  kind_actions: Record<string, PrivacyMaskingAction>
  // 智能脱敏（Presidio 旁路）：超时或不可用时自动降级为仅规则脱敏
  smart_masking_enabled: boolean
  smart_sidecar_url: string
  smart_timeout_ms: number
  smart_min_score: number
  smart_entities: string[]
  updated_at?: string
  updated_by_user_id?: number
}

export interface PrivacyMaskingValueSample {
  kind: string
  prefix: string
  length: number
}

export interface PrivacyMaskingLog {
  id: number
  request_id?: string
  user_id: number
  api_key_id?: number
  group_id?: number
  detected_at: string
  kinds: Record<string, number>
  actions: Record<string, PrivacyMaskingAction>
  total_count: number
  masked_count: number
  warned_count: number
  blocked_count: number
  request_blocked: boolean
  value_samples?: PrivacyMaskingValueSample[]
  endpoint?: string
  model?: string
  created_at: string
}

export interface PrivacyMaskingLogQuery {
  page?: number
  page_size?: number
  endpoint?: string
  kind?: string
  model?: string
  user_id?: number
  api_key_id?: number
  group_id?: number
  blocked?: string
  start_time?: string
  end_time?: string
}

export interface PrivacyMaskingEndpointSummary {
  endpoint: string
  event_count: number
  blocked_count: number
  detected_count: number
  masked_count: number
  warned_count: number
  last_seen_at: string
}

export interface PrivacyMaskingStats {
  since: string
  event_count: number
  blocked_count: number
  detected_count: number
  masked_count: number
  warned_count: number
  by_kind: Record<string, number>
}

export async function getConfig(): Promise<PrivacyMaskingConfig> {
  const { data } = await apiClient.get('/admin/privacy-masking/config')
  return data
}

export async function updateConfig(config: PrivacyMaskingConfig): Promise<PrivacyMaskingConfig> {
  const { data } = await apiClient.put('/admin/privacy-masking/config', config)
  return data
}

export async function listLogs(params: PrivacyMaskingLogQuery): Promise<PaginatedResponse<PrivacyMaskingLog>> {
  const { data } = await apiClient.get('/admin/privacy-masking/logs', { params })
  return data
}

export async function listEndpoints(params: {
  start_time?: string
  end_time?: string
}): Promise<PrivacyMaskingEndpointSummary[]> {
  const { data } = await apiClient.get('/admin/privacy-masking/logs/endpoints', { params })
  return data ?? []
}

export async function getStats(since?: string): Promise<PrivacyMaskingStats> {
  const { data } = await apiClient.get('/admin/privacy-masking/stats', { params: { since } })
  return data
}

export interface PrivacyMaskingSmartCheckResult {
  ok: boolean
  url: string
  latency_ms: number
  error?: string
}

/** 检测智能脱敏旁路是否可达；url 为空时检测已保存的地址（或环境变量） */
export async function checkSmartSidecar(url?: string): Promise<PrivacyMaskingSmartCheckResult> {
  const { data } = await apiClient.post('/admin/privacy-masking/smart/check', { url: url || '' })
  return data
}

export const privacyMaskingAPI = {
  getConfig,
  updateConfig,
  listLogs,
  listEndpoints,
  getStats,
  checkSmartSidecar
}

export default privacyMaskingAPI
