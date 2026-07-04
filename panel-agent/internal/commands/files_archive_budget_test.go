package commands

import "testing"

// GH #652: the reservation must be race-safe — reserving up to the budget
// succeeds, one more fails, and a release frees a slot. (Assumes no stale
// jabali-archive scratch files in /tmp; the reaper + CI isolation hold that.)
func TestReserveArchiveBudget_BoundedAndReleases(t *testing.T) {
	archiveInFlightMu.Lock()
	saved := archiveReservedBytes
	archiveReservedBytes = 0
	archiveInFlightMu.Unlock()
	defer func() {
		archiveInFlightMu.Lock()
		archiveReservedBytes = saved
		archiveInFlightMu.Unlock()
	}()

	slots := int(maxArchiveStagingBytes / maxArchiveBytes)
	for i := 0; i < slots; i++ {
		if !reserveArchiveBudget() {
			t.Fatalf("reservation %d/%d should have succeeded", i+1, slots)
		}
	}
	// Budget now fully reserved — the next one must fail.
	if reserveArchiveBudget() {
		t.Fatal("over-budget reservation should have been refused")
	}
	// Releasing one frees a slot.
	releaseArchiveBudget()
	if !reserveArchiveBudget() {
		t.Fatal("reservation after a release should succeed")
	}
}
