export default {
  privacyMasking: {
    title: 'Privacy Masking',
    description: 'Auto-detect and mask sensitive data before sending to upstream, with masking logs per API endpoint',

    // Tabs
    tabs: {
      rules: 'Rules',
      logs: 'Masking Logs',
    },

    // Global toggle
    enabled: {
      title: 'Enable Privacy Masking',
      description: 'Auto-detect and mask sensitive data before sending to upstream',
      hint: 'When disabled, all masking rules stop working and requests are forwarded directly. Changes apply immediately after saving.',
    },

    // Basic settings
    basic: {
      title: 'Basic Settings',
      maskSecrets: {
        title: 'Mask Secrets',
        description: 'Auto-detect and mask API keys, private keys, tokens, passwords',
      },
      maskPersonal: {
        title: 'Mask Personal Data',
        description: 'Auto-detect and mask emails, phone numbers, credit cards',
      },
    },

    // Action settings
    actions: {
      title: 'Types & Actions',
      description: 'Choose which types to detect and the action for each: mask and forward, warn only (forward original), or block the request',
      mask: 'Mask (Recommended)',
      warn: 'Warn Only',
      block: 'Block Request',
      kinds: {
        API_KEY: {
          label: 'API Keys',
          description: 'OpenAI, Anthropic and other API keys',
        },
        PRIVATE_KEY: {
          label: 'Private Keys',
          description: 'RSA, SSH private keys',
        },
        TOKEN: {
          label: 'Tokens',
          description: 'JWT, GitHub tokens',
        },
        PASSWORD: {
          label: 'Passwords',
          description: 'Password fields',
        },
        EMAIL: {
          label: 'Emails',
          description: 'Email addresses',
        },
        PHONE: {
          label: 'Phone Numbers',
          description: 'Phone numbers',
        },
        CREDIT_CARD: {
          label: 'Credit Cards',
          description: 'Credit card numbers',
        },
        CUSTOM: {
          label: 'Custom',
          description: 'Matched by a custom rule',
        },
      },
    },

    // Allow list
    allowList: {
      title: 'Allow List',
      description: 'These values will never be masked (one per line, exact match, case-insensitive)',
      placeholder: 'example-key-12345\ntest@example.com\n+1234567890',
      hint: 'Add known safe values like test keys',
    },

    // Custom rules
    customRules: {
      title: 'Custom Rules',
      description: 'Define custom masking rules using regular expressions; with capture groups only the first group is replaced',
      pattern: 'Regular Expression',
      action: 'Action',
      add: '+ Add Rule',
      remove: 'Remove',
      patternPlaceholder: 'e.g. secret-[a-zA-Z0-9]+',
      kindPlaceholder: 'Kind, e.g. CUSTOM',
    },

    // Logs
    logs: {
      title: 'Masking Logs',
      refresh: 'Refresh',
      empty: 'No masking logs yet',
      disabledHint: 'Privacy masking is currently disabled, so the gateway produces no new masking logs. Enable it on the Rules tab.',
      unknownEndpoint: '(unknown endpoint)',
      endpoints: {
        title: 'By API Endpoint',
        hint: 'Click a row to filter logs by that endpoint',
        events: 'Events',
        blocked: 'Blocked',
        detected: 'Values hit',
        lastSeen: 'Last seen',
      },
      stats: {
        events: 'Masking events',
        blocked: 'Blocked requests',
        masked: 'Values masked',
        warned: 'Values warned',
      },
      filters: {
        all: 'All',
        allEndpoints: 'All endpoints',
        endpoint: 'API endpoint',
        kind: 'Type',
        result: 'Result',
        model: 'Model',
        userId: 'User ID',
        apiKeyId: 'API Key ID',
      },
      columns: {
        time: 'Time',
        endpoint: 'Endpoint / Model',
        caller: 'Caller',
        kinds: 'Types',
        count: 'Count',
        action: 'Handling',
        status: 'Status',
        requestId: 'Request ID',
      },
      caller: {
        user: 'User',
        apiKey: 'Key',
        group: 'Group',
      },
      actions: {
        masked: 'Masked',
        warned: 'Warned',
        blocked: 'Blocked',
      },
      status: {
        blocked: 'Blocked',
        passed: 'Passed',
      },
    },

    // Buttons
    buttons: {
      save: 'Save Configuration',
      saving: 'Saving...',
    },

    // Messages
    messages: {
      saveSuccess: 'Configuration saved',
      saveError: 'Failed to save',
      loadError: 'Failed to load configuration',
      loadLogsError: 'Failed to load masking logs',
    },

    // Help text
    help: {
      howItWorks: {
        title: 'How It Works',
        description: 'Masking feature automatically detects and replaces sensitive data with placeholders (like {{API_KEY_hash}}) before sending to upstream API. Original values are restored when responses return. Raw values are stored in memory only, never written to disk.',
      },
      placeholders: {
        title: 'Placeholder Format',
        description: 'Sensitive values are replaced with {{KIND_hash}} format placeholders, e.g. {{API_KEY_a3b4c5d6}}. Same value always generates same placeholder to ensure context consistency.',
      },
      safety: {
        title: 'Safety First',
        description: 'If masking fails, request will be blocked and not sent to upstream, ensuring sensitive data never leaks.',
      },
    },
  },
}
