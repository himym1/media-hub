package qms

import "testing"

func TestPublicSyncFailureMapsAuthorizationExpiry(t *testing.T) {
	code, message := PublicSyncFailure("115账号授权失效，请在网盘账号管理中重新授权")
	if code != CodeSyncAuthExpired || message != messageAuthExpired {
		t.Fatalf("code=%q message=%q", code, message)
	}
}

func TestPublicSyncFailureKeepsSafeReason(t *testing.T) {
	code, message := PublicSyncFailure("本地目录不可写")
	if code != CodeSyncFailed || message != messageSyncFailed+"：本地目录不可写" {
		t.Fatalf("code=%q message=%q", code, message)
	}
}

func TestPublicSyncFailureDropsSecretsAndPaths(t *testing.T) {
	code, message := PublicSyncFailure("cookie=secret /volume1/docker/media")
	if code != CodeSyncFailed || message != messageSyncFailed {
		t.Fatalf("code=%q message=%q", code, message)
	}
}

func TestAccountAuthDetailReportsExpired115(t *testing.T) {
	detail := accountAuthDetail([]cloudAccount{{
		ID: 1, SourceType: "115", HasToken: false, TokenFailedReason: "115账号授权失效，请在网盘账号管理中重新授权",
	}})
	if detail != messageAuthDegraded {
		t.Fatalf("detail=%q", detail)
	}
}

func TestAccountAuthDetailIgnoresHealthy115(t *testing.T) {
	detail := accountAuthDetail([]cloudAccount{{
		ID: 1, SourceType: "115", HasToken: true,
	}})
	if detail != "" {
		t.Fatalf("detail=%q", detail)
	}
}
