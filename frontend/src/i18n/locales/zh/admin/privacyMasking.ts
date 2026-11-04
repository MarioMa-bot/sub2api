export default {
  privacyMasking: {
    title: '隐私脱敏',
    description: '在请求发送到上游前自动检测并脱敏敏感信息',
    
    // 全局开关
    enabled: {
      title: '启用隐私脱敏',
      description: '在请求发送到上游前自动检测并脱敏敏感信息',
      hint: '关闭后所有脱敏规则停止工作，请求将直接转发',
    },
    
    // 基础配置
    basic: {
      title: '基础配置',
      maskSecrets: {
        title: '脱敏密钥和令牌',
        description: '自动检测并脱敏 API 密钥、私钥、令牌、密码等敏感凭证',
      },
      maskPersonal: {
        title: '脱敏个人信息',
        description: '自动检测并脱敏邮箱、电话、信用卡号等个人身份信息',
      },
    },
    
    // 动作配置
    actions: {
      title: '动作配置',
      description: '对检测到的每种敏感信息采取不同的动作',
      mask: '脱敏 (推荐)',
      warn: '仅警告',
      block: '阻止请求',
      kinds: {
        API_KEY: {
          label: 'API 密钥',
          description: 'OpenAI、Anthropic 等 API 密钥',
        },
        PRIVATE_KEY: {
          label: '私钥',
          description: 'RSA、SSH 私钥',
        },
        TOKEN: {
          label: '令牌',
          description: 'JWT、GitHub token 等',
        },
        PASSWORD: {
          label: '密码',
          description: '密码字段',
        },
        EMAIL: {
          label: '邮箱',
          description: '电子邮箱地址',
        },
        PHONE: {
          label: '电话',
          description: '电话号码',
        },
        CREDIT_CARD: {
          label: '信用卡',
          description: '信用卡号',
        },
      },
    },
    
    // 白名单
    allowList: {
      title: '白名单',
      description: '这些值永远不会被脱敏（每行一个，精确匹配）',
      placeholder: 'example-key-12345\ntest@example.com\n+1234567890',
      hint: '添加已知安全的值，如测试密钥',
    },
    
    // 自定义规则
    customRules: {
      title: '自定义规则',
      description: '使用正则表达式定义自定义的脱敏规则',
      pattern: '正则表达式',
      action: '动作',
      add: '+ 添加规则',
      remove: '删除',
      patternPlaceholder: '例如: secret-[a-zA-Z0-9]+',
    },
    
    // 日志
    logs: {
      title: '最近的脱敏日志',
      refresh: '刷新',
      empty: '暂无脱敏日志',
      columns: {
        time: '时间',
        kinds: '类型',
        count: '数量',
        action: '动作',
        status: '状态',
      },
      actions: {
        masked: '已脱敏',
        warned: '已警告',
        blocked: '已阻止',
      },
      status: {
        blocked: '已阻止',
        passed: '已通过',
      },
    },
    
    // 按钮和操作
    buttons: {
      save: '保存配置',
      saving: '保存中...',
    },
    
    // 消息
    messages: {
      saveSuccess: '配置已保存',
      saveError: '保存失败',
      loadError: '加载配置失败',
    },
    
    // 帮助文本
    help: {
      howItWorks: {
        title: '工作原理',
        description: '脱敏功能会在请求发送到上游 API 前自动检测并替换敏感信息为占位符（如 {{API_KEY_hash}}），响应返回时再将占位符恢复为原始值。原始值仅存储在内存中，不会写入磁盘。',
      },
      placeholders: {
        title: '占位符格式',
        description: '敏感值会被替换为 {{KIND_hash}} 格式的占位符，例如 {{API_KEY_a3b4c5d6}}。相同的值总是生成相同的占位符，保证上下文一致性。',
      },
      safety: {
        title: '安全保障',
        description: '如果脱敏过程失败，请求将被直接阻止而不会发送到上游，确保敏感信息不会泄露。',
      },
    },
  },
}
