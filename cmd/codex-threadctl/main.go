package main

import (
	"bufio"
	"bytes"
	"context"
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

const version = "0.3.0"

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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "list":
		err = runList(os.Args[2:])
	case "read":
		err = runRead(os.Args[2:])
	case "create":
		err = runCreate(os.Args[2:])
	case "send":
		err = runSend(os.Args[2:])
	case "rename":
		err = runRename(os.Args[2:])
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
  codex-threadctl read --id THREAD_ID [--json]
  codex-threadctl create --cwd PATH --title TITLE (--message TEXT|--message-file PATH) [--json]
  codex-threadctl send --id THREAD_ID (--message TEXT|--message-file PATH) [--cwd PATH] [--json]
  codex-threadctl rename --id THREAD_ID --name TITLE [--expect-current TITLE] [--dry-run|--confirm]
  codex-threadctl version

Examples:
  codex-threadctl list --search Project --limit 1
  codex-threadctl list --cwd /absolute/project/root --limit 100
  codex-threadctl read --id 019...
  codex-threadctl create --cwd /absolute/project/root --title 'LE | Naomi | Coordinator' --message-file kickoff.md
  codex-threadctl send --id 019... --message 'Status request'
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

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "thread cwd")
	title := fs.String("title", "", "thread title")
	message := fs.String("message", "", "kickoff message text")
	messageFile := fs.String("message-file", "", "path to kickoff message text")
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

	session, err := startSession(60 * time.Minute)
	if err != nil {
		return err
	}
	defer session.close()

	raw, err := session.request("thread/start", map[string]any{
		"cwd":          *cwd,
		"threadSource": "vscode",
		"ephemeral":    false,
	})
	if err != nil {
		return err
	}
	var start threadStartResult
	if err := json.Unmarshal(raw, &start); err != nil {
		return err
	}
	if start.ThreadID == "" {
		start.ThreadID = start.Thread.ID
	}
	if start.ThreadID == "" {
		return fmt.Errorf("thread/start returned no threadId: %s", string(raw))
	}

	if _, err := session.request("thread/name/set", map[string]any{
		"threadId": start.ThreadID,
		"name":     *title,
	}); err != nil {
		return fmt.Errorf("thread created as %s but title set failed: %w", start.ThreadID, err)
	}

	turnStatus, err := session.startTurn(start.ThreadID, *cwd, text)
	if err != nil {
		return fmt.Errorf("thread created as %s but turn start failed: %w", start.ThreadID, err)
	}

	if *asJSON {
		return printJSON(map[string]any{
			"threadId": start.ThreadID,
			"title":    *title,
			"cwd":      *cwd,
			"turn":     turnStatus,
		})
	}
	fmt.Printf("created\t%s\n", start.ThreadID)
	fmt.Printf("title\t%s\n", *title)
	fmt.Printf("cwd\t%s\n", *cwd)
	fmt.Printf("turn\t%s\n", turnStatus)
	fmt.Printf("directive\t::created-thread{threadId=%q}\n", start.ThreadID)
	return nil
}

func runSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	cwd := fs.String("cwd", "", "optional cwd override; defaults to thread cwd")
	message := fs.String("message", "", "message text")
	messageFile := fs.String("message-file", "", "path to message text")
	asJSON := fs.Bool("json", false, "emit JSON result")
	if err := fs.Parse(args); err != nil {
		return err
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
	session, err := startSession(60 * time.Minute)
	if err != nil {
		return err
	}
	defer session.close()

	thread, err := session.resumeThread(*id)
	if err != nil {
		return fmt.Errorf("resume before send failed: %w", err)
	}
	targetCwd := *cwd
	if targetCwd == "" {
		targetCwd = thread.Cwd
	}
	if targetCwd == "" {
		return errors.New("send requires --cwd because target thread has no cwd")
	}

	turnStatus, err := session.startTurn(*id, targetCwd, text)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(map[string]any{
			"threadId": *id,
			"cwd":      targetCwd,
			"turn":     turnStatus,
		})
	}
	fmt.Printf("sent\t%s\n", *id)
	fmt.Printf("cwd\t%s\n", targetCwd)
	fmt.Printf("turn\t%s\n", turnStatus)
	return nil
}

func runRename(args []string) error {
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	id := fs.String("id", "", "thread id")
	name := fs.String("name", "", "new title")
	expectCurrent := fs.String("expect-current", "", "optional current title guard")
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
	fmt.Printf("renamed\t%s\n", *id)
	fmt.Printf("before\t%s\n", before.Name)
	fmt.Printf("after\t%s\n", after.Name)
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

func (s *appSession) startTurn(threadID, cwd, text string) (string, error) {
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
	return s.waitForTurn(threadID)
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

func printThread(t threadSummary) {
	name := t.Name
	if name == "" {
		name = "<no title>"
	}
	fmt.Printf("id\t%s\n", t.ID)
	fmt.Printf("title\t%s\n", name)
	fmt.Printf("cwd\t%s\n", t.Cwd)
	fmt.Printf("source\t%s\n", t.Source)
	fmt.Printf("updatedAt\t%d\n", t.UpdatedAt)
	if t.Preview != "" {
		fmt.Printf("preview\t%s\n", compact(t.Preview, 240))
	}
}
