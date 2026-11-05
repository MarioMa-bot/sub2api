package service

import (
	"context"
	"strings"
	"testing"
)

func TestPrivacyMaskingService_Mask(t *testing.T) {
	ctx := context.Background()
	svc, err := NewPrivacyMaskingService(nil)
	if err != nil {
		t.Fatalf("NewPrivacyMaskingService failed: %v", err)
	}

	// 启用脱敏
	svc.applyConfig(&PrivacyMaskingConfig{
		Enabled:      true,
		MaskSecrets:  true,
		MaskPersonal: false,
		MaskedKinds:  make(map[string]bool),
		CustomRules:  []PrivacyMaskingRule{},
		AllowList:    []string{},
		KindActions:  make(map[string]PrivacyMaskingAction),
	})

	tests := []struct {
		name             string
		input            string
		shouldContain    string
		shouldNotContain string
	}{
		{
			name:             "OpenAI API Key",
			input:            "My API key is sk-1234567890abcdefghij",
			shouldContain:    "{{API_KEY_",
			shouldNotContain: "sk-1234567890abcdefghij",
		},
		{
			name:             "Anthropic API Key",
			input:            "Using sk-ant-api03-1234567890",
			shouldContain:    "{{API_KEY_",
			shouldNotContain: "sk-ant-api03-1234567890",
		},
		{
			name:          "No sensitive data",
			input:         "Hello world",
			shouldContain: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.Mask(ctx, tt.input)
			if err != nil {
				t.Fatalf("Mask failed: %v", err)
			}

			if tt.shouldContain != "" && !strings.Contains(result.Masked, tt.shouldContain) {
				t.Errorf("Expected masked text to contain %q, got %q", tt.shouldContain, result.Masked)
			}

			if tt.shouldNotContain != "" && strings.Contains(result.Masked, tt.shouldNotContain) {
				t.Errorf("Expected masked text not to contain %q, got %q", tt.shouldNotContain, result.Masked)
			}
		})
	}
}

func TestPrivacyMaskingService_MaskUnmask_Roundtrip(t *testing.T) {
	ctx := context.Background()
	svc, err := NewPrivacyMaskingService(nil)
	if err != nil {
		t.Fatalf("NewPrivacyMaskingService failed: %v", err)
	}

	svc.applyConfig(&PrivacyMaskingConfig{
		Enabled:      true,
		MaskSecrets:  true,
		MaskPersonal: false,
		MaskedKinds:  make(map[string]bool),
		CustomRules:  []PrivacyMaskingRule{},
		AllowList:    []string{},
		KindActions:  make(map[string]PrivacyMaskingAction),
	})

	original := "My API key is sk-1234567890abcdefghij and another sk-9876543210zyxwvutsrq"

	// Mask
	result, err := svc.Mask(ctx, original)
	if err != nil {
		t.Fatalf("Mask failed: %v", err)
	}

	if result.Masked == original {
		t.Error("Expected text to be masked")
	}

	if result.MaskedCount != 2 {
		t.Errorf("Expected 2 masked values, got %d", result.MaskedCount)
	}

	// Unmask
	unmasked := UnmaskWith(result.Placeholders, result.Masked)

	if unmasked != original {
		t.Errorf("Unmask failed: expected %q, got %q", original, unmasked)
	}
}

func TestPrivacyMaskingService_AllowList(t *testing.T) {
	ctx := context.Background()
	svc, err := NewPrivacyMaskingService(nil)
	if err != nil {
		t.Fatalf("NewPrivacyMaskingService failed: %v", err)
	}

	svc.applyConfig(&PrivacyMaskingConfig{
		Enabled:      true,
		MaskSecrets:  true,
		MaskPersonal: false,
		MaskedKinds:  make(map[string]bool),
		CustomRules:  []PrivacyMaskingRule{},
		AllowList:    []string{"sk-test1234567890abcdef"},
		KindActions:  make(map[string]PrivacyMaskingAction),
	})

	input := "My keys: sk-test1234567890abcdef and sk-prod1234567890abcdef"
	result, err := svc.Mask(ctx, input)
	if err != nil {
		t.Fatalf("Mask failed: %v", err)
	}

	// 白名单的应该保留
	if !strings.Contains(result.Masked, "sk-test1234567890abcdef") {
		t.Error("Allowed value should not be masked")
	}

	// 非白名单的应该脱敏
	if strings.Contains(result.Masked, "sk-prod1234567890abcdef") {
		t.Error("Non-allowed value should be masked")
	}
}

func TestPrivacyMaskingService_Actions(t *testing.T) {
	ctx := context.Background()
	svc, err := NewPrivacyMaskingService(nil)
	if err != nil {
		t.Fatalf("NewPrivacyMaskingService failed: %v", err)
	}

	svc.applyConfig(&PrivacyMaskingConfig{
		Enabled:      true,
		MaskSecrets:  true,
		MaskPersonal: false,
		MaskedKinds:  make(map[string]bool),
		CustomRules: []PrivacyMaskingRule{
			{Pattern: `secret-\w+`, Kind: KindCustom, Action: PrivacyMaskingActionBlock},
		},
		AllowList:   []string{},
		KindActions: make(map[string]PrivacyMaskingAction),
	})

	input := "This contains secret-abc123"
	result, err := svc.Mask(ctx, input)
	if err != nil {
		t.Fatalf("Mask failed: %v", err)
	}

	if !result.ShouldBlock {
		t.Error("Expected request to be blocked")
	}

	if result.BlockedCount != 1 {
		t.Errorf("Expected 1 blocked value, got %d", result.BlockedCount)
	}
}
