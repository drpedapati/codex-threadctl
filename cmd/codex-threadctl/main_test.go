package main

import (
	"reflect"
	"testing"
	"time"
)

func TestAuditThreadFlagsCanonicalAndRiskSignals(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	thread := threadSummary{
		ID:        "019-test",
		Name:      "LE-T | Naomi | Control Tower",
		Cwd:       "/work/project",
		Source:    "vscode",
		UpdatedAt: now.Add(-48 * time.Hour).Unix(),
		Preview:   "Recovery probe after smoke-send visibility check.",
	}

	entry := auditThread(thread, auditOptions{
		ExpectTitle: "LE-T | Naomi | Control Tower",
		ExpectCwd:   "/work/project",
		StaleAfter:  24 * time.Hour,
	}, now)

	want := []string{"canonical_title", "canonical_cwd", "recovery", "probe", "stale"}
	if !reflect.DeepEqual(entry.Flags, want) {
		t.Fatalf("flags = %#v, want %#v", entry.Flags, want)
	}
	if entry.LastActivity != "2026-06-29T12:00:00Z" {
		t.Fatalf("last activity = %q", entry.LastActivity)
	}
}

func TestAuditThreadFlagsMismatchesAndMissingMetadata(t *testing.T) {
	entry := auditThread(threadSummary{
		ID: "019-test",
	}, auditOptions{
		ExpectTitle: "Expected Title",
		ExpectCwd:   "/work/project",
	}, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))

	want := []string{"missing_title", "missing_cwd", "title_mismatch", "cwd_mismatch"}
	if !reflect.DeepEqual(entry.Flags, want) {
		t.Fatalf("flags = %#v, want %#v", entry.Flags, want)
	}
	if entry.LastActivity != "" {
		t.Fatalf("last activity = %q, want empty", entry.LastActivity)
	}
}

func TestAuditThreadRoleMapFlags(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	entry := auditThread(threadSummary{
		ID:   "019-old",
		Name: "ARCHIVE CANDIDATE | LE-M | Mara | VM1 Build & Runtime Evidence STALE",
		Cwd:  "/work/project",
	}, auditOptions{
		RoleMap: map[string]roleMapThread{
			"019-old": {
				Role:     "mara-vm1-deploy",
				Status:   "stale-broken",
				Relation: "role_previous",
			},
		},
	}, now)

	want := []string{"archive_candidate", "role_previous", "role_mara_vm1_deploy", "role_status_stale_broken"}
	if !reflect.DeepEqual(entry.Flags, want) {
		t.Fatalf("flags = %#v, want %#v", entry.Flags, want)
	}
}

func TestAuditThreadRoleUnmapped(t *testing.T) {
	entry := auditThread(threadSummary{
		ID:   "019-probe",
		Name: "Probe thread",
	}, auditOptions{
		RoleMap: map[string]roleMapThread{},
	}, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))

	want := []string{"missing_cwd", "probe", "role_unmapped"}
	if !reflect.DeepEqual(entry.Flags, want) {
		t.Fatalf("flags = %#v, want %#v", entry.Flags, want)
	}
}

func TestThreadTimeAcceptsSecondsAndMilliseconds(t *testing.T) {
	seconds := int64(1782828739)
	millis := seconds * 1000

	if got, want := threadTime(seconds), time.Unix(seconds, 0).UTC(); !got.Equal(want) {
		t.Fatalf("seconds time = %s, want %s", got, want)
	}
	if got, want := threadTime(millis), time.UnixMilli(millis).UTC(); !got.Equal(want) {
		t.Fatalf("milliseconds time = %s, want %s", got, want)
	}
}

func TestSanitizeFlag(t *testing.T) {
	if got, want := sanitizeFlag("mara/vm1-leading-edge deploy"), "mara_vm1_leading_edge_deploy"; got != want {
		t.Fatalf("sanitizeFlag = %q, want %q", got, want)
	}
}
