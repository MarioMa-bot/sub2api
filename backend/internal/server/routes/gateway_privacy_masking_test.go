package routes

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGatewayRoutesPrivacyMaskingMountedOnEveryGatewayChain 断言所有网关入口都挂载了隐私脱敏，
// 且位于分组校验之后（被拒绝的请求不产生脱敏日志）。
func TestGatewayRoutesPrivacyMaskingMountedOnEveryGatewayChain(t *testing.T) {
	routeSource, err := os.ReadFile("gateway.go")
	require.NoError(t, err)
	source := string(routeSource)

	for _, chain := range []struct{ group, requireGroup string }{
		{"gateway", "requireGroupAnthropic"},
		{"gemini", "requireGroupGoogle"},
		{"antigravityV1", "requireGroupAnthropic"},
		{"antigravityV1Beta", "requireGroupGoogle"},
	} {
		re := regexp.MustCompile(regexp.QuoteMeta(chain.group+".Use("+chain.requireGroup+")") + `\s*` +
			regexp.QuoteMeta(chain.group+".Use(privacyMasking)"))
		require.Regexp(t, re, source, "%s chain must mount privacyMasking right after %s", chain.group, chain.requireGroup)
	}
	require.Contains(t, source, "requireGroupAnthropic, privacyMasking, handler)", "rootRoute helper must mount privacyMasking")
	require.Contains(t, source, "codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), groupModelAllowlist, compositeTarget, requireGroupAnthropic, privacyMasking)")
}
