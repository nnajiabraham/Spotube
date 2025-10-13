package sqliteconn

import "testing"

func TestOpenWithPragmas(t *testing.T) {
	db, err := OpenWithPragmas("test.db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	if db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("expected MaxOpenConns 1, got %d", db.Stats().MaxOpenConnections)
	}
}
