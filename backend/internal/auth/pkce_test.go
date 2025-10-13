package auth

import "testing"

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(verifier) == 0 {
		t.Fatalf("verifier should not be empty")
	}
}

func TestCodeChallenge(t *testing.T) {
	verifier := "test-verifier"
	challenge := CodeChallenge(verifier)
	if challenge == "" {
		t.Fatalf("challenge should not be empty")
	}
	if challenge == verifier {
		t.Fatalf("challenge should differ from verifier")
	}
}
