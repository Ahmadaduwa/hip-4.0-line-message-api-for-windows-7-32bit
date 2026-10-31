package storage

import (
	"os"
	"testing"
)

func TestTelegramRecipientsStorage(t *testing.T) {
	tgRecipientsFile = "data/test_tg_recipients.json"
	tgDiscoveredFile = "data/test_tg_discovered.json"
	defer func() {
		_ = os.Remove(tgRecipientsFile)
		_ = os.Remove(tgDiscoveredFile)
	}()

	rec, isNew := UpsertTelegramUser(1001, "Test User", "private", "testuser", true)
	if !isNew || rec.ID != 1001 || !rec.RealtimeOn || !rec.SummaryOn {
		t.Fatalf("unexpected new recipient: %+v", rec)
	}

	// Toggle
	if !ToggleTelegramRecipient(1001, "realtime", false) {
		t.Fatalf("failed to toggle realtime")
	}

	rIDs := GetTelegramRealtimeIDs()
	if len(rIDs) != 0 {
		t.Fatalf("expected 0 realtime IDs, got %v", rIDs)
	}

	sIDs := GetTelegramSummaryIDs()
	if len(sIDs) != 1 || sIDs[0] != 1001 {
		t.Fatalf("expected 1 summary ID, got %v", sIDs)
	}

	// Delete
	if !RemoveTelegramRecipient(1001) {
		t.Fatalf("failed to remove recipient")
	}

	if len(GetTelegramRecipients()) != 0 {
		t.Fatalf("expected empty recipients after removal")
	}
}

