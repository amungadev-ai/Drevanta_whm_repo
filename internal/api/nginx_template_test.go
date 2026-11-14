package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// G40-S3 §4 regression: the shipped nginx template must terminate every
// browser-facing path — including /api/v1/ — at the SvelteKit BFF. Routing
// /api/v1/ straight to the Go control plane bypasses the BFF's token->cookie
// ceremony and hands raw bearer tokens to the page (browser-evidenced defect
// fixed in G40-S2; this test pins the correction permanently).
func TestNginxTemplateRoutesBrowserTrafficThroughBFF(t *testing.T) {
	path := filepath.Join("..", "..", "infra", "nginx", "drevanta-panel-vhost.conf.example")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue // commentary may cite the historical defect
		}
		if strings.Contains(trimmed, "proxy_pass") && strings.Contains(trimmed, ":8080") {
			t.Fatalf("template routes traffic directly to the Go control plane (%q); "+
				"browser paths must terminate at the BFF only", trimmed)
		}
		if strings.HasPrefix(trimmed, "location /api/v1/") {
			t.Fatalf("template declares a dedicated %q location; /api/v1/ must follow the "+
				"generic BFF location so the BFF performs the token->cookie ceremony", trimmed)
		}
	}
	if !strings.Contains(string(raw), "proxy_pass http://127.0.0.1:3100") {
		t.Fatalf("template must proxy to the SvelteKit BFF on 127.0.0.1:3100")
	}
}
