package referral_test

import (
	"strings"
	"testing"

	"github.com/eduplexo/backend-go/internal/domain/referral"
)

func TestGenerateReferralToken(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok := referral.GenerateReferralToken()
		if !strings.HasPrefix(tok, "PUB_") {
			t.Fatalf("expected token to start with PUB_, got %s", tok)
		}
		if len(tok) != 12 { // "PUB_" (4) + 8 chars = 12
			t.Fatalf("expected token length 12, got %d for token %s", len(tok), tok)
		}
		if tokens[tok] {
			t.Fatalf("generated duplicate token %s within 100 iterations", tok)
		}
		tokens[tok] = true
	}
}

func TestGeneratePublisherID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 50; i++ {
		id := referral.GeneratePublisherID()
		if !strings.HasPrefix(id, "pub_") {
			t.Fatalf("expected ID to start with pub_, got %s", id)
		}
		if len(id) != 16 { // "pub_" (4) + 12 chars = 16
			t.Fatalf("expected ID length 16, got %d for ID %s", len(id), id)
		}
		if ids[id] {
			t.Fatalf("generated duplicate ID %s", id)
		}
		ids[id] = true
	}
}

func TestBuildReferralURL(t *testing.T) {
	token := "PUB_TEST1234"
	url := referral.BuildReferralURL(token)
	expected := "https://app.eduplexo.com/auth/register?ref=PUB_TEST1234"
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}
}
