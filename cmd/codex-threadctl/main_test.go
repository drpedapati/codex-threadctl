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
