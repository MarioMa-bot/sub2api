import { describe, expect, it } from 'vitest'
import { baseCompile } from '@intlify/message-compiler'
import { createI18n } from 'vue-i18n'

import en from '../locales/en/admin/privacyMasking'
import zh from '../locales/zh/admin/privacyMasking'

function leaves(value: unknown, prefix = ''): [string, string][] {
  if (typeof value === 'string') return [[prefix, value]]
  if (value === null || typeof value !== 'object') return []
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    leaves(child, prefix ? `${prefix}.${key}` : key)
  )
}

// 邮箱里的 @、占位符 {{KIND_hash}} 等是 vue-i18n 语法字符，未转义会导致整页渲染失败
describe('privacy masking i18n messages', () => {
  for (const [locale, messages] of Object.entries({ en, zh })) {
    it(`compiles every ${locale} message`, () => {
      const failures: string[] = []
      for (const [key, message] of leaves(messages)) {
        baseCompile(message, {
          onError: (err) => failures.push(`${key}: ${err.message}`)
        })
      }
      expect(failures).toEqual([])
    })
  }

  it('renders escaped characters literally', () => {
    const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: { admin: en } } })
    expect(i18n.global.t('admin.privacyMasking.allowList.placeholder')).toContain('test@example.com')
    expect(i18n.global.t('admin.privacyMasking.help.placeholders.description')).toContain('{{KIND_hash}}')
  })
})
