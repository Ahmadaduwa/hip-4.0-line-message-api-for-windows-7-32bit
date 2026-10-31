package storage

import (
	"os"
	"testing"
)

func TestAddOrActivateRecipient(t *testing.T) {
	// Use test files
	recipientsFile = "data/test_recipients.json"
	discoveredFile = "data/test_discovered.json"
	defer func() {
		_ = os.Remove(recipientsFile)
		_ = os.Remove(discoveredFile)
	}()

	testID := "U12345678901234567890123456789012"
	rec, isNew := AddOrActivateRecipient(testID, "ครูทดสอบ")
	if !isNew {
		t.Fatalf("expected isNew=true on first add")
	}
	if rec.ID != testID || rec.Name != "ครูทดสอบ" || !rec.Active {
		t.Fatalf("unexpected recipient: %+v", rec)
	}

	// Deactivate
	ok := ToggleRecipient(testID, false)
	if !ok {
		t.Fatalf("failed to toggle")
	}

	// Add again - should reactivate
	rec2, isNew2 := AddOrActivateRecipient(testID, "ครูทดสอบ (อัปเดต)")
	if isNew2 {
		t.Fatalf("expected isNew=false on reactivate")
	}
	if !rec2.Active || rec2.Name != "ครูทดสอบ (อัปเดต)" {
		t.Fatalf("expected reactivated and name updated, got %+v", rec2)
	}
}
