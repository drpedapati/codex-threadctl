package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const version = "0.8.0"

const leadingEdgeCwd = "/Users/ernie/Documents/GitHub/clinvision-v2-leading-edge-worktrees"

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	Method string          `json:"method"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type threadListResult struct {
	Data []threadSummary `json:"data"`
}

type threadReadResult struct {
	Thread threadSummary `json:"thread"`
}

type threadFullReadResult struct {
	Thread threadFull `json:"thread"`
}

type threadFull struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Preview   string        `json:"preview"`
	Cwd       string        `json:"cwd"`
	Source    string        `json:"source"`
	CreatedAt int64         `json:"createdAt"`
	UpdatedAt int64         `json:"updatedAt"`
	Turns     []turnSummary `json:"turns"`
}

type turnSummary struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	StartedAt   int64      `json:"startedAt"`
	CompletedAt int64      `json:"completedAt"`
	DurationMs  int64      `json:"durationMs"`
	Items       []turnItem `json:"items"`
}

type turnItem struct {
	Type    string        `json:"type"`
	ID      string        `json:"id"`
	Text    string        `json:"text"`
	Phase   string        `json:"phase"`
	Content []messagePart `json:"content"`
}

type messagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type threadStartResult struct {
	ThreadID string `json:"threadId"`
	Thread   struct {
		ID string `json:"id"`
	} `json:"thread"`
}

type threadSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Preview   string `json:"preview"`
	Cwd       string `json:"cwd"`
	Source    string `json:"source"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type auditEntry struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Cwd          string   `json:"cwd"`
	Source       string   `json:"source"`
	UpdatedAt    int64    `json:"updatedAt"`
	LastActivity string   `json:"lastActivity,omitempty"`
	Flags        []string `json:"flags"`
	Preview      string   `json:"preview,omitempty"`
}

type auditOptions struct {
	ExpectTitle string
	ExpectCwd   string
	StaleAfter  time.Duration
	RoleMap     map[string]roleMapThread
}

type roleMapThread struct {
	Role     string
	Status   string
	Relation string
}

type roleWorktreeMap struct {
	Entries []roleWorktreeEntry `json:"entries"`
}

type roleWorktreeEntry struct {
	Role                 string `json:"role"`
	Status               string `json:"status"`
	ThreadID             string `json:"thread_id"`
	PreviousThreadID     string `json:"previous_thread_id"`
	PreviousThreadStatus string `json:"previous_thread_status"`
}

type deliveryVerification struct {
	Verified       bool   `json:"verified"`
	Status         string `json:"status"`
	TurnID         string `json:"turnId,omitempty"`
	TurnStatus     string `json:"turnStatus,omitempty"`
	MessageSHA256  string `json:"message_sha256"`
	LastUserSHA256 string `json:"last_user_sha256,omitempty"`
	LastUserSample string `json:"lastUserSample,omitempty"`
}

type roundtripVerification struct {
	UserVerified        bool   `json:"userVerified"`
	AssistantVerified   bool   `json:"assistantVerified"`
	RoundtripVerified   bool   `json:"roundtripVerified"`
	Status              string `json:"status"`
	TurnID              string `json:"turnId,omitempty"`
	TurnStatus          string `json:"turnStatus,omitempty"`
	LastUserSample      string `json:"lastUserSample,omitempty"`
	LastAssistantSample string `json:"lastAssistantSample,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "list":
		err = runList(os.Args[2:])
	case "audit":
		err = runAudit(os.Args[2:])
	case "read":
		err = runRead(os.Args[2:])
	case "last":
		err = runLast(os.Args[2:])
	case "create":
		err = runCreate(os.Args[2:])
	case "le-create":
		err = runLECreate(os.Args[2:])
	case "send":
		err = runSend(os.Args[2:])
	case "smoke-send":
		err = runSmokeSend(os.Args[2:])
	case "rename":
		err = runRename(os.Args[2:])
	case "doctor":
		err = runDoctor(os.Args[2:])
	case "version":
		fmt.Println(version)
		return
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  codex-threadctl list [--search TERM] [--cwd PATH] [--limit N] [--json]
  codex-threadctl audit [--search TERM] [--cwd PATH] [--limit N] [--expect-title TITLE] [--expect-cwd PATH] [--stale-after DURATION] [--role-map PATH] [--json]
  codex-threadctl read --id THREAD_ID [--json]
  codex-threadctl last --id THREAD_ID [--json]
  codex-threadctl create --cwd PATH --title TITLE (--message TEXT|--message-file PATH) [--receipt PATH] [--json]
  codex-threadctl le-create --role ROLE --lane LANE (--message TEXT|--message-file PATH) [--cwd PATH] [--receipt PATH] [--json]
  codex-threadctl send --id THREAD_ID (--message TEXT|--message-file PATH) [--expect-title TITLE] [--expect-cwd PATH] [--cwd PATH] [--receipt PATH] [--wait-timeout DURATION|--no-wait] [--json]
  codex-threadctl smoke-send --id THREAD_ID [--marker MARKER] [--expect-title TITLE] [--expect-cwd PATH] [--cwd PATH] [--receipt PATH] [--wait-timeout DURATION] [--json]
  codex-threadctl rename --id THREAD_ID --name TITLE [--expect-current TITLE] [--receipt PATH] [--dry-run|--confirm]
  codex-threadctl doctor [--json]
  codex-threadctl version

Examples:
  codex-threadctl list --search Project --limit 1
  codex-threadctl list --cwd /absolute/project/root --limit 100
  codex-threadctl audit --search Naomi --expect-title 'LE-T | Naomi | Control Tower' --expect-cwd /absolute/project/root --stale-after 168h --role-map role-worktree-map.json
  codex-threadctl read --id 019...
  codex-threadctl last --id 019...
  codex-threadctl create --cwd /absolute/project/root --title 'LE | Naomi | Coordinator' --message-file kickoff.md
  codex-threadctl le-create --role Naomi --lane 'Project Coordinator Manager' --message-file kickoff.md
  codex-threadctl send --id 019... --expect-title 'LE | Naomi | Project Coordinator Manager' --expect-cwd /absolute/project/root --message 'Status request'
  codex-threadctl smoke-send --id 019... --marker THREADCTL_SMOKE_20260701T003516Z
  codex-threadctl rename --id 019... --name 'V2 | Role | PR #123 - Short Lane' --expect-current 'V2 | Role | Old Lane' --dry-run
  codex-threadctl rename --id 019... --name 'V2 | Role | PR #123 - Short Lane' --expect-current 'V2 | Role | Old Lane' --confirm`)
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	search := fs.String("search", "", "substring search term")
	cwd := fs.String("cwd", "", "exact cwd filter")
	limit := fs.Int("limit", 20, "maximum threads to return")
	asJSON := fs.Bool("json", false, "emit raw JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}

	params := map[string]any{
		"limit": uint32(*limit),
	}
	if *search != "" {
		params["searchTerm"] = *search
	}
	if *cwd != "" {
		params["cwd"] = *cwd
	}

	raw, err := call("thread/list", params)
	if err != nil {
		return err
	}
	if *asJSON {
		fmt.Println(string(raw))
		return nil
	}

	var result threadListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	for _, t := range result.Data {
		name := t.Name
		if name == "" {
			name = "<no title>"
		}
		preview := compact(t.Preview, 160)
		fmt.Printf("%s\t%s\t%s\t%s\n", t.ID, name, t.Cwd, preview)
	}
	return nil
}

func runAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	search := fs.String("search", "", "substring search term")
	cwd := fs.String("cwd", "", "exact cwd filter")
	limit := fs.Int("limit", 50, "maximum threads to return")
	expectTitle := fs.String("expect-title", "", "optional expected canonical title")
	expectCwd := fs.String("expect-cwd", "", "optional expected canonical cwd")
	staleAfter := fs.Duration("stale-after", 0, "flag threads with no activity after this duration, for example 168h")
	roleMapPath := fs.String("role-map", "", "optional role-worktree-map.json path for role-aware audit flags")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit <= 0 {
		return errors.New("audit requires --limit > 0")
	}
	if *staleAfter < 0 {
		return errors.New("audit requires --stale-after >= 0")
	}

	params := map[string]any{
		"limit": uint32(*limit),
	}
	if *search != "" {
		params["searchTerm"] = *search
	}
	if *cwd != "" {
		params["cwd"] = *cwd
	}

	raw, err := call("thread/list", params)
	if err != nil {
		return err
	}
	var result threadListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}

	roleMap := map[string]roleMapThread(nil)
	if *roleMapPath != "" {
		roleMap, err = loadRoleMap(*roleMapPath)
		if err != nil {
			return err
		}
	}

	now := time.Now()
	opts := auditOptions{
		ExpectTitle: *expectTitle,
		ExpectCwd:   *expectCwd,
		StaleAfter:  *staleAfter,
		RoleMap:     roleMap,
	}
	entries := make([]auditEntry, 0, len(result.Data))
	for _, t := range result.Data {
		entries = append(entries, auditThread(t, opts, now))
	}
	if *asJSON {
		return printJSON(map[string]any{
			"threads": entries,
			"count":   len(entries),
		})
	}
	for _, entry := range entries {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.ID,
			displayName(entry.Title),
			entry.Cwd,
			entry.Source,
			entry.LastActivity,
			strings.Join(entry.Flags, ","),
			compact(entry.Preview, 120),
		)
	}
	return nil
}

func runRead(args []string) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	asJSON := fs.Bool("json", false, "emit raw JSON response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("read requires --id")
	}

	raw, err := call("thread/read", map[string]any{
		"threadId":     *id,
		"includeTurns": false,
	})
	if err != nil {
		return err
	}
	if *asJSON {
		fmt.Println(string(raw))
		return nil
	}

	var result threadReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	printThread(result.Thread)
	return nil
}

func runLast(args []string) error {
	fs := flag.NewFlagSet("last", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("last requires --id")
	}

	raw, err := call("thread/resume", map[string]any{
		"threadId":     *id,
		"includeTurns": true,
	})
	if err != nil {
		return err
	}
	var result threadFullReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	last := summarizeLastTurn(result.Thread)
	if *asJSON {
		return printJSON(last)
	}
	fmt.Printf("id\t%s\n", result.Thread.ID)
	fmt.Printf("title\t%s\n", displayName(result.Thread.Name))
	fmt.Printf("cwd\t%s\n", result.Thread.Cwd)
	if last["turnId"] != nil {
		fmt.Printf("turnId\t%s\n", last["turnId"])
		fmt.Printf("status\t%s\n", last["status"])
		fmt.Printf("completedAt\t%v\n", last["completedAt"])
		fmt.Printf("durationMs\t%v\n", last["durationMs"])
	}
	if msg, ok := last["lastUserMessage"].(string); ok && msg != "" {
		fmt.Printf("lastUserMessage\t%s\n", compact(msg, 400))
	}
	if msg, ok := last["lastAssistantMessage"].(string); ok && msg != "" {
		fmt.Printf("lastAssistantMessage\t%s\n", compact(msg, 400))
	}
	return nil
}

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "thread cwd")
	title := fs.String("title", "", "thread title")
	message := fs.String("message", "", "kickoff message text")
	messageFile := fs.String("message-file", "", "path to kickoff message text")
	receiptPath := fs.String("receipt", "", "optional JSON receipt path")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cwd == "" || *title == "" {
		return errors.New("create requires --cwd and --title")
	}
	text, err := readMessage(*message, *messageFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("create requires a non-empty --message or --message-file")
	}

	result, err := createThread(*cwd, *title, text)
	if err != nil {
		return err
	}
	if *receiptPath != "" {
		if err := writeReceipt(*receiptPath, "create", result); err != nil {
			return err
		}
	}
	if *asJSON {
		return printJSON(result)
	}
	fmt.Printf("created\t%s\n", result["threadId"])
	fmt.Printf("title\t%s\n", result["title"])
	fmt.Printf("cwd\t%s\n", result["cwd"])
	fmt.Printf("turn\t%s\n", result["turn"])
	fmt.Printf("directive\t::created-thread{threadId=%q}\n", result["threadId"])
	return nil
}

func runLECreate(args []string) error {
	fs := flag.NewFlagSet("le-create", flag.ContinueOnError)
	role := fs.String("role", "", "LE role name")
	lane := fs.String("lane", "", "LE lane name")
	cwd := fs.String("cwd", leadingEdgeCwd, "thread cwd")
	message := fs.String("message", "", "kickoff message text")
	messageFile := fs.String("message-file", "", "path to kickoff message text")
	receiptPath := fs.String("receipt", "", "optional JSON receipt path")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *role == "" || *lane == "" {
		return errors.New("le-create requires --role and --lane")
	}
	title := fmt.Sprintf("LE | %s | %s", strings.TrimSpace(*role), strings.TrimSpace(*lane))
	text, err := readMessage(*message, *messageFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("le-create requires a non-empty --message or --message-file")
	}

	result, err := createThread(*cwd, title, text)
	if err != nil {
		return err
	}
	result["role"] = *role
	result["lane"] = *lane
	if *receiptPath != "" {
		if err := writeReceipt(*receiptPath, "le-create", result); err != nil {
			return err
		}
	}
	if *asJSON {
		return printJSON(result)
	}
	fmt.Printf("created\t%s\n", result["threadId"])
	fmt.Printf("title\t%s\n", result["title"])
	fmt.Printf("cwd\t%s\n", result["cwd"])
	fmt.Printf("turn\t%s\n", result["turn"])
	fmt.Printf("directive\t::created-thread{threadId=%q}\n", result["threadId"])
	return nil
}

func createThread(cwd, title, text string) (map[string]any, error) {
	session, err := startSession(60 * time.Minute)
	if err != nil {
		return nil, err
	}
	defer session.close()

	raw, err := session.request("thread/start", map[string]any{
		"cwd":          cwd,
		"threadSource": "vscode",
		"ephemeral":    false,
	})
	if err != nil {
		return nil, err
	}
	var start threadStartResult
	if err := json.Unmarshal(raw, &start); err != nil {
		return nil, err
	}
	if start.ThreadID == "" {
		start.ThreadID = start.Thread.ID
	}
	if start.ThreadID == "" {
		return nil, fmt.Errorf("thread/start returned no threadId: %s", string(raw))
	}

	if _, err := session.request("thread/name/set", map[string]any{
		"threadId": start.ThreadID,
		"name":     title,
	}); err != nil {
		return nil, fmt.Errorf("thread created as %s but title set failed: %w", start.ThreadID, err)
	}

	turnStatus, err := session.startTurn(start.ThreadID, cwd, text)
	if err != nil {
		return nil, fmt.Errorf("thread created as %s but turn start failed: %w", start.ThreadID, err)
	}
	return map[string]any{
		"threadId": start.ThreadID,
		"title":    title,
		"cwd":      cwd,
		"turn":     turnStatus,
	}, nil
}

func runSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	cwd := fs.String("cwd", "", "optional cwd override; defaults to thread cwd")
	expectTitle := fs.String("expect-title", "", "optional target title guard")
	expectCwd := fs.String("expect-cwd", "", "optional target cwd guard")
	message := fs.String("message", "", "message text")
	messageFile := fs.String("message-file", "", "path to message text")
	receiptPath := fs.String("receipt", "", "optional JSON receipt path")
	waitTimeout := fs.Duration("wait-timeout", 10*time.Minute, "maximum time to wait for turn completion")
	noWait := fs.Bool("no-wait", false, "start the turn and return without waiting for completion")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *noWait && *waitTimeout != 10*time.Minute {
		return errors.New("send accepts --no-wait or --wait-timeout, not both")
	}
	if *waitTimeout < 0 {
		return errors.New("send requires --wait-timeout >= 0")
	}
	if *id == "" {
		return errors.New("send requires --id")
	}
	text, err := readMessage(*message, *messageFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("send requires a non-empty --message or --message-file")
	}
	sessionTimeout := *waitTimeout + 30*time.Second
	if *noWait {
		sessionTimeout = 60 * time.Second
	}
	if sessionTimeout < 60*time.Second {
		sessionTimeout = 60 * time.Second
	}
	session, err := startSession(sessionTimeout)
	if err != nil {
		return err
	}
	defer session.close()

	thread, err := session.resumeThread(*id)
	if err != nil {
		return fmt.Errorf("resume before send failed: %w", err)
	}
	if err := checkThreadGuards(thread, *expectTitle, *expectCwd); err != nil {
		return err
	}
	targetCwd := *cwd
	if targetCwd == "" {
		targetCwd = thread.Cwd
	}
	if targetCwd == "" {
		return errors.New("send requires --cwd because target thread has no cwd")
	}

	turnStatus, err := session.startTurnWithOptions(*id, targetCwd, text, !*noWait, *waitTimeout)
	if err != nil {
		return err
	}
	digest := messageDigest(text)
	result := map[string]any{
		"threadId":       *id,
		"title":          thread.Name,
		"cwd":            targetCwd,
		"turn":           turnStatus,
		"message_sha256": digest,
	}
	if *noWait {
		result["status"] = "request_started"
		result["delivery_verified"] = false
		result["delivery_status"] = "verification_skipped_no_wait"
	} else {
		verification, err := session.verifyLatestUserMessage(*id, text)
		if err != nil {
			result["status"] = "delivery_unverified"
			result["delivery_verified"] = false
			result["delivery_status"] = "readback_failed"
			result["delivery_error"] = err.Error()
		} else {
			result["delivery"] = verification
			result["delivery_verified"] = verification.Verified
			result["delivery_status"] = verification.Status
			if verification.Verified {
				result["status"] = "delivery_verified"
			} else {
				result["status"] = "delivery_unverified"
			}
		}
	}
	if *receiptPath != "" {
		if err := writeReceipt(*receiptPath, "send", result); err != nil {
			return err
		}
	}
	if *asJSON {
		if err := printJSON(result); err != nil {
			return err
		}
		if result["status"] == "delivery_unverified" {
			return errors.New("delivery_unverified")
		}
		return nil
	}
	fmt.Printf("%s\t%s\n", result["status"], *id)
	fmt.Printf("title\t%s\n", displayName(thread.Name))
	fmt.Printf("cwd\t%s\n", targetCwd)
	fmt.Printf("turn\t%s\n", turnStatus)
	fmt.Printf("message_sha256\t%s\n", digest)
	fmt.Printf("delivery_verified\t%t\n", result["delivery_verified"])
	if status, _ := result["delivery_status"].(string); status != "" {
		fmt.Printf("delivery_status\t%s\n", status)
	}
	if *noWait {
		fmt.Fprintln(os.Stderr, "note: --no-wait only means request_started; it is not durable delivery")
	}
	if turnStatus == "wait_timeout" || turnStatus == "interrupted" || turnStatus == "started" {
		fmt.Fprintln(os.Stderr, "note: turn status is not work success; run codex-threadctl last and verify repo/PR/evidence truth")
	}
	if result["status"] == "delivery_unverified" {
		return errors.New("delivery_unverified")
	}
	return nil
}

func runSmokeSend(args []string) error {
	fs := flag.NewFlagSet("smoke-send", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	cwd := fs.String("cwd", "", "optional cwd override; defaults to thread cwd")
	expectTitle := fs.String("expect-title", "", "optional target title guard")
	expectCwd := fs.String("expect-cwd", "", "optional target cwd guard")
	marker := fs.String("marker", "", "marker text; generated when omitted")
	receiptPath := fs.String("receipt", "", "optional JSON receipt path")
	waitTimeout := fs.Duration("wait-timeout", 2*time.Minute, "maximum time to wait for assistant ACK")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("smoke-send requires --id")
	}
	if *waitTimeout <= 0 {
		return errors.New("smoke-send requires --wait-timeout > 0")
	}
	smokeMarker := strings.TrimSpace(*marker)
	if smokeMarker == "" {
		smokeMarker = "THREADCTL_SMOKE_" + time.Now().UTC().Format("20060102T150405Z")
	}
	text := fmt.Sprintf("codex-threadctl smoke-send marker: %s\n\nPlease reply with exactly: ACK %s", smokeMarker, smokeMarker)
	sessionTimeout := *waitTimeout + 30*time.Second
	if sessionTimeout < 60*time.Second {
		sessionTimeout = 60 * time.Second
	}
	session, err := startSession(sessionTimeout)
	if err != nil {
		return err
	}
	defer session.close()

	thread, err := session.resumeThread(*id)
	if err != nil {
		return fmt.Errorf("resume before smoke-send failed: %w", err)
	}
	if err := checkThreadGuards(thread, *expectTitle, *expectCwd); err != nil {
		return err
	}
	targetCwd := *cwd
	if targetCwd == "" {
		targetCwd = thread.Cwd
	}
	if targetCwd == "" {
		return errors.New("smoke-send requires --cwd because target thread has no cwd")
	}

	turnStatus, err := session.startTurnWithOptions(*id, targetCwd, text, true, *waitTimeout)
	if err != nil {
		return err
	}
	verification, err := session.verifyLatestRoundtrip(*id, smokeMarker)
	result := map[string]any{
		"threadId":       *id,
		"title":          thread.Name,
		"cwd":            targetCwd,
		"marker":         smokeMarker,
		"turn":           turnStatus,
		"expected_ack":   "ACK " + smokeMarker,
		"message_sha256": messageDigest(text),
	}
	if err != nil {
		result["status"] = "fail"
		result["delivery_verified"] = false
		result["roundtrip_verified"] = false
		result["delivery_status"] = "readback_failed"
		result["delivery_error"] = err.Error()
	} else {
		result["delivery"] = verification
		result["delivery_verified"] = verification.UserVerified
		result["roundtrip_verified"] = verification.RoundtripVerified
		result["delivery_status"] = verification.Status
		if verification.RoundtripVerified {
			result["status"] = "pass"
		} else {
			result["status"] = "fail"
		}
	}
	if *receiptPath != "" {
		if err := writeReceipt(*receiptPath, "smoke-send", result); err != nil {
			return err
		}
	}
	if *asJSON {
		if err := printJSON(result); err != nil {
			return err
		}
		if result["status"] != "pass" {
			return errors.New("smoke-send failed")
		}
		return nil
	}
	fmt.Printf("status\t%s\n", result["status"])
	fmt.Printf("threadId\t%s\n", *id)
	fmt.Printf("title\t%s\n", displayName(thread.Name))
	fmt.Printf("cwd\t%s\n", targetCwd)
	fmt.Printf("turn\t%s\n", turnStatus)
	fmt.Printf("marker\t%s\n", smokeMarker)
	fmt.Printf("delivery_verified\t%t\n", result["delivery_verified"])
	fmt.Printf("roundtrip_verified\t%t\n", result["roundtrip_verified"])
	if status, _ := result["delivery_status"].(string); status != "" {
		fmt.Printf("delivery_status\t%s\n", status)
	}
	if result["status"] != "pass" {
		return errors.New("smoke-send failed")
	}
	return nil
}

func runRename(args []string) error {
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	name := fs.String("name", "", "new title")
	expectCurrent := fs.String("expect-current", "", "optional current title guard")
	receiptPath := fs.String("receipt", "", "optional JSON receipt path")
	dryRun := fs.Bool("dry-run", false, "show intended rename without mutating")
	confirm := fs.Bool("confirm", false, "required to mutate thread metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *name == "" {
		return errors.New("rename requires --id and --name")
	}
	before, err := readThread(*id)
	if err != nil {
		return fmt.Errorf("read before rename failed: %w", err)
	}
	if *expectCurrent != "" && before.Name != *expectCurrent {
		return fmt.Errorf("current title mismatch: expected %q, got %q", *expectCurrent, before.Name)
	}
	if *dryRun {
		fmt.Printf("dry-run\t%s\n", *id)
		fmt.Printf("before\t%s\n", before.Name)
		fmt.Printf("after\t%s\n", *name)
		return nil
	}
	if !*confirm {
		return errors.New("refusing to rename without --confirm")
	}

	_, err = call("thread/name/set", map[string]any{
		"threadId": *id,
		"name":     *name,
	})
	if err != nil {
		return err
	}
	after, err := readThread(*id)
	if err != nil {
		return fmt.Errorf("rename sent but readback failed: %w", err)
	}
	if after.Name != *name {
		return fmt.Errorf("rename readback mismatch: expected %q, got %q", *name, after.Name)
	}
	if *receiptPath != "" {
		if err := writeReceipt(*receiptPath, "rename", map[string]any{
			"threadId": *id,
			"before":   before.Name,
			"after":    after.Name,
			"cwd":      after.Cwd,
		}); err != nil {
			return err
		}
	}
	fmt.Printf("renamed\t%s\n", *id)
	fmt.Printf("before\t%s\n", before.Name)
	fmt.Printf("after\t%s\n", after.Name)
	return nil
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checks := []map[string]any{}
	ok := true
	add := func(name string, pass bool, detail string) {
		if !pass {
			ok = false
		}
		checks = append(checks, map[string]any{
			"name":   name,
			"status": map[bool]string{true: "pass", false: "fail"}[pass],
			"detail": detail,
		})
	}
	codexPath, err := exec.LookPath("codex")
	add("codex_on_path", err == nil, codexPath)
	if err == nil {
		session, err := startSession(15 * time.Second)
		if err != nil {
			add("app_server_initialize", false, err.Error())
		} else {
			add("app_server_initialize", true, "initialized")
			_, err = session.request("thread/list", map[string]any{"limit": uint32(1)})
			add("thread_list", err == nil, errorDetail(err, "list ok"))
			session.close()
		}
	}
	result := map[string]any{
		"version": version,
		"ok":      ok,
		"checks":  checks,
	}
	if *asJSON {
		return printJSON(result)
	}
	fmt.Printf("version\t%s\n", version)
	fmt.Printf("ok\t%t\n", ok)
	for _, check := range checks {
		fmt.Printf("%s\t%s\t%s\n", check["name"], check["status"], check["detail"])
	}
	if !ok {
		return errors.New("doctor failed")
	}
	return nil
}

func readMessage(inline, file string) (string, error) {
	if inline != "" && file != "" {
		return "", errors.New("use either --message or --message-file, not both")
	}
	if file == "" {
		return inline, nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func checkThreadGuards(thread threadSummary, expectTitle, expectCwd string) error {
	if expectTitle != "" && thread.Name != expectTitle {
		return fmt.Errorf("target title mismatch: expected %q, got %q", expectTitle, thread.Name)
	}
	if expectCwd != "" && thread.Cwd != expectCwd {
		return fmt.Errorf("target cwd mismatch: expected %q, got %q", expectCwd, thread.Cwd)
	}
	return nil
}

func writeReceipt(path, action string, payload map[string]any) error {
	receipt := map[string]any{
		"schema":      "codex-threadctl.receipt.v1",
		"version":     version,
		"action":      action,
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"payload":     payload,
		"tool_origin": "codex-threadctl",
	}
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func summarizeLastTurn(thread threadFull) map[string]any {
	result := map[string]any{
		"threadId": thread.ID,
		"title":    thread.Name,
		"cwd":      thread.Cwd,
	}
	if len(thread.Turns) == 0 {
		return result
	}
	turn := thread.Turns[len(thread.Turns)-1]
	result["turnId"] = turn.ID
	result["status"] = turn.Status
	result["startedAt"] = turn.StartedAt
	result["completedAt"] = turn.CompletedAt
	result["durationMs"] = turn.DurationMs
	for _, item := range turn.Items {
		switch item.Type {
		case "userMessage":
			if text := itemText(item); text != "" {
				result["lastUserMessage"] = text
			}
		case "agentMessage":
			if text := itemText(item); text != "" {
				result["lastAssistantMessage"] = text
			}
		}
	}
	return result
}

func itemText(item turnItem) string {
	if item.Text != "" {
		return item.Text
	}
	parts := []string{}
	for _, part := range item.Content {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func messageDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func textSample(text string) string {
	return compact(strings.ReplaceAll(text, "\n", "\\n"), 240)
}

func errorDetail(err error, ok string) string {
	if err == nil {
		return ok
	}
	return err.Error()
}

func loadRoleMap(path string) (map[string]roleMapThread, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read role map: %w", err)
	}
	var roleMap roleWorktreeMap
	if err := json.Unmarshal(b, &roleMap); err != nil {
		return nil, fmt.Errorf("parse role map: %w", err)
	}
	threads := make(map[string]roleMapThread)
	for _, entry := range roleMap.Entries {
		if entry.ThreadID != "" {
			threads[entry.ThreadID] = roleMapThread{
				Role:     entry.Role,
				Status:   entry.Status,
				Relation: "role_current",
			}
		}
		if entry.PreviousThreadID != "" {
			status := entry.PreviousThreadStatus
			if status == "" {
				status = entry.Status
			}
			threads[entry.PreviousThreadID] = roleMapThread{
				Role:     entry.Role,
				Status:   status,
				Relation: "role_previous",
			}
		}
	}
	return threads, nil
}

func auditThread(t threadSummary, opts auditOptions, now time.Time) auditEntry {
	flags := []string{}
	title := strings.TrimSpace(t.Name)
	cwd := strings.TrimSpace(t.Cwd)
	if title == "" {
		flags = append(flags, "missing_title")
	}
	if cwd == "" {
		flags = append(flags, "missing_cwd")
	}
	if opts.ExpectTitle != "" {
		if t.Name == opts.ExpectTitle {
			flags = append(flags, "canonical_title")
		} else {
			flags = append(flags, "title_mismatch")
		}
	}
	if opts.ExpectCwd != "" {
		if t.Cwd == opts.ExpectCwd {
			flags = append(flags, "canonical_cwd")
		} else {
			flags = append(flags, "cwd_mismatch")
		}
	}
	text := strings.ToLower(t.Name + " " + t.Preview)
	if strings.Contains(text, "recover") || strings.Contains(text, "recovery") {
		flags = append(flags, "recovery")
	}
	if strings.Contains(text, "probe") || strings.Contains(text, "smoke") || strings.Contains(text, "visibility") {
		flags = append(flags, "probe")
	}
	last := threadTime(t.UpdatedAt)
	if opts.StaleAfter > 0 && !last.IsZero() && now.Sub(last) > opts.StaleAfter {
		flags = append(flags, "stale")
	}
	if strings.HasPrefix(strings.ToLower(title), "archive candidate") {
		flags = append(flags, "archive_candidate")
	}
	if opts.RoleMap != nil {
		if roleThread, ok := opts.RoleMap[t.ID]; ok {
			flags = append(flags, roleThread.Relation)
			if roleThread.Role != "" {
				flags = append(flags, "role_"+sanitizeFlag(roleThread.Role))
			}
			if roleThread.Status != "" {
				flags = append(flags, "role_status_"+sanitizeFlag(roleThread.Status))
			}
		} else {
			flags = append(flags, "role_unmapped")
		}
	}
	if len(flags) == 0 {
		flags = append(flags, "ok")
	}
	return auditEntry{
		ID:           t.ID,
		Title:        t.Name,
		Cwd:          t.Cwd,
		Source:       t.Source,
		UpdatedAt:    t.UpdatedAt,
		LastActivity: formatThreadTime(last),
		Flags:        flags,
		Preview:      compact(t.Preview, 240),
	}
}

func sanitizeFlag(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func threadTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	if v > 1_000_000_000_000 {
		return time.UnixMilli(v).UTC()
	}
	return time.Unix(v, 0).UTC()
}

func formatThreadTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func readThread(id string) (threadSummary, error) {
	raw, err := call("thread/read", map[string]any{
		"threadId":     id,
		"includeTurns": false,
	})
	if err != nil {
		return threadSummary{}, err
	}
	var result threadReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return threadSummary{}, err
	}
	return result.Thread, nil
}

type appSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr *bytes.Buffer
	nextID int
}

func startSession(timeout time.Duration) (*appSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	s := &appSession{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
		stderr: stderr,
		nextID: 1,
	}
	_, err = s.request("initialize", map[string]any{
		"clientInfo": map[string]any{
			"name":    "codex-threadctl",
			"version": version,
		},
		"capabilities": map[string]any{
			"experimentalApi": true,
		},
	})
	if err != nil {
		s.close()
		cancel()
		return nil, err
	}
	return s, nil
}

func (s *appSession) request(method string, params any) (json.RawMessage, error) {
	id := s.nextID
	s.nextID++
	if err := s.write(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return nil, err
	}
	for s.stdout.Scan() {
		var resp rpcResponse
		if err := json.Unmarshal(s.stdout.Bytes(), &resp); err != nil {
			continue
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
	if err := s.stdout.Err(); err != nil {
		return nil, err
	}
	if s.stderr.Len() > 0 {
		return nil, fmt.Errorf("no response from codex app-server\nstderr: %s", strings.TrimSpace(s.stderr.String()))
	}
	return nil, errors.New("no response from codex app-server")
}

func (s *appSession) readThread(id string) (threadSummary, error) {
	raw, err := s.request("thread/read", map[string]any{
		"threadId":     id,
		"includeTurns": false,
	})
	if err != nil {
		return threadSummary{}, err
	}
	var result threadReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return threadSummary{}, err
	}
	return result.Thread, nil
}

func (s *appSession) resumeThread(id string) (threadSummary, error) {
	raw, err := s.request("thread/resume", map[string]any{
		"threadId":     id,
		"includeTurns": false,
	})
	if err != nil {
		return threadSummary{}, err
	}
	var result threadReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return threadSummary{}, err
	}
	return result.Thread, nil
}

func (s *appSession) resumeFullThread(id string) (threadFull, error) {
	raw, err := call("thread/resume", map[string]any{
		"threadId":     id,
		"includeTurns": true,
	})
	if err != nil {
		return threadFull{}, err
	}
	var result threadFullReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return threadFull{}, err
	}
	return result.Thread, nil
}

func (s *appSession) verifyLatestUserMessage(threadID, expected string) (deliveryVerification, error) {
	thread, err := s.resumeFullThread(threadID)
	if err != nil {
		return deliveryVerification{}, err
	}
	expectedDigest := messageDigest(expected)
	verification := deliveryVerification{
		Status:        "no_turns",
		MessageSHA256: expectedDigest,
	}
	if len(thread.Turns) == 0 {
		return verification, nil
	}
	turn := thread.Turns[len(thread.Turns)-1]
	verification.TurnID = turn.ID
	verification.TurnStatus = turn.Status
	for _, item := range turn.Items {
		if item.Type != "userMessage" {
			continue
		}
		text := itemText(item)
		if text == "" {
			continue
		}
		lastDigest := messageDigest(text)
		verification.LastUserSHA256 = lastDigest
		verification.LastUserSample = textSample(text)
		if lastDigest == expectedDigest {
			verification.Verified = true
			verification.Status = "latest_user_message_verified"
		} else {
			verification.Status = "latest_user_message_mismatch"
		}
	}
	if verification.LastUserSHA256 == "" {
		verification.Status = "latest_turn_missing_user_message"
	}
	return verification, nil
}

func (s *appSession) verifyLatestRoundtrip(threadID, marker string) (roundtripVerification, error) {
	thread, err := s.resumeFullThread(threadID)
	if err != nil {
		return roundtripVerification{}, err
	}
	expectedAck := "ACK " + marker
	verification := roundtripVerification{
		Status: "no_turns",
	}
	if len(thread.Turns) == 0 {
		return verification, nil
	}
	turn := thread.Turns[len(thread.Turns)-1]
	verification.TurnID = turn.ID
	verification.TurnStatus = turn.Status
	for _, item := range turn.Items {
		text := itemText(item)
		if text == "" {
			continue
		}
		switch item.Type {
		case "userMessage":
			verification.LastUserSample = textSample(text)
			if strings.Contains(text, marker) {
				verification.UserVerified = true
			}
		case "agentMessage":
			verification.LastAssistantSample = textSample(text)
			if strings.Contains(text, expectedAck) {
				verification.AssistantVerified = true
			}
		}
	}
	verification.RoundtripVerified = verification.UserVerified && verification.AssistantVerified
	switch {
	case verification.RoundtripVerified:
		verification.Status = "roundtrip_verified"
	case !verification.UserVerified && !verification.AssistantVerified:
		verification.Status = "marker_missing_from_user_and_assistant"
	case !verification.UserVerified:
		verification.Status = "marker_missing_from_user_message"
	case !verification.AssistantVerified:
		verification.Status = "ack_missing_from_assistant_message"
	}
	return verification, nil
}

func (s *appSession) startTurn(threadID, cwd, text string) (string, error) {
	return s.startTurnWithOptions(threadID, cwd, text, true, 0)
}

func (s *appSession) startTurnWithOptions(threadID, cwd, text string, wait bool, waitTimeout time.Duration) (string, error) {
	if _, err := s.request("turn/start", map[string]any{
		"threadId": threadID,
		"cwd":      cwd,
		"input": []map[string]any{
			{
				"type":          "text",
				"text":          text,
				"text_elements": []any{},
			},
		},
	}); err != nil {
		return "", err
	}
	if !wait {
		return "started", nil
	}
	if waitTimeout > 0 {
		return s.waitForTurnWithTimeout(threadID, waitTimeout)
	}
	return s.waitForTurn(threadID)
}

type turnWaitResult struct {
	status string
	err    error
}

func (s *appSession) waitForTurnWithTimeout(threadID string, timeout time.Duration) (string, error) {
	done := make(chan turnWaitResult, 1)
	go func() {
		status, err := s.waitForTurn(threadID)
		done <- turnWaitResult{status: status, err: err}
	}()

	select {
	case result := <-done:
		return result.status, result.err
	case <-time.After(timeout):
		return "wait_timeout", nil
	}
}

func (s *appSession) waitForTurn(threadID string) (string, error) {
	for s.stdout.Scan() {
		var resp rpcResponse
		if err := json.Unmarshal(s.stdout.Bytes(), &resp); err != nil {
			continue
		}
		if resp.Method == "" {
			continue
		}
		if resp.Method == "turn/completed" {
			return "completed", nil
		}
		if resp.Method == "turn/failed" || resp.Method == "turn/error" {
			return "failed", nil
		}
		if resp.Method == "turn/interrupted" || resp.Method == "turn/cancelled" || resp.Method == "turn/canceled" {
			return "interrupted", nil
		}
	}
	if err := s.stdout.Err(); err != nil {
		return "", err
	}
	if s.stderr.Len() > 0 {
		return "", fmt.Errorf("turn status unavailable\nstderr: %s", strings.TrimSpace(s.stderr.String()))
	}
	return "", errors.New("turn status unavailable")
}

func (s *appSession) write(req rpcRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(s.stdin, string(b))
	return err
}

func (s *appSession) close() {
	_ = s.stdin.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	_, _ = s.cmd.Process.Wait()
}

func call(method string, params any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	write := func(req rpcRequest) error {
		b, err := json.Marshal(req)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdin, string(b))
		return err
	}

	if err := write(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"clientInfo": map[string]any{
				"name":    "codex-threadctl",
				"version": version,
			},
			"capabilities": map[string]any{
				"experimentalApi": true,
			},
		},
	}); err != nil {
		return nil, err
	}

	const callID = 2
	if err := write(rpcRequest{
		JSONRPC: "2.0",
		ID:      callID,
		Method:  method,
		Params:  params,
	}); err != nil {
		return nil, err
	}

	result, err := readResponse(stdout, callID)
	_ = stdin.Close()
	if killErr := cmd.Process.Kill(); killErr == nil {
		_, _ = cmd.Process.Wait()
	}
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return result, nil
}

func readResponse(r io.Reader, targetID int) (json.RawMessage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var resp rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.ID != targetID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("json-rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("no response from codex app-server")
}

func compact(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}
	if limit <= 1 {
		return s[:limit]
	}
	return s[:limit-1] + "…"
}

func displayName(name string) string {
	if name == "" {
		return "<no title>"
	}
	return name
}

func printThread(t threadSummary) {
	fmt.Printf("id\t%s\n", t.ID)
	fmt.Printf("title\t%s\n", displayName(t.Name))
	fmt.Printf("cwd\t%s\n", t.Cwd)
	fmt.Printf("source\t%s\n", t.Source)
	fmt.Printf("updatedAt\t%d\n", t.UpdatedAt)
	if t.Preview != "" {
		fmt.Printf("preview\t%s\n", compact(t.Preview, 240))
	}
}
