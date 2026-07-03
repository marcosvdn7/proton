package signup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// batchServer returns responses keyed by the Name query parameter.
// If a name isn't in the map it responds with 12106 (taken) and no suggestions.
func batchServer(t *testing.T, byName map[string]availabilityResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("Name")
		resp, ok := byName[name]
		if !ok {
			resp = availabilityResponse{Code: 12106, Error: "taken"}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func availResp(code int, suggestions ...string) availabilityResponse {
	r := availabilityResponse{Code: code}
	r.Details.Suggestions = suggestions
	return r
}

func TestCheckBatch_EmptyInputErrors(t *testing.T) {
	var buf bytes.Buffer
	_, err := checkBatchWith(nil, false, http.DefaultClient, "http://unused", &buf)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCheckBatch_HumanOutputPreservesOrder(t *testing.T) {
	srv := batchServer(t, map[string]availabilityResponse{
		"free1": availResp(1000),
		"taken": availResp(12106, "taken7", "taken_dev"),
		"free2": availResp(1000),
	})
	defer srv.Close()

	var buf bytes.Buffer
	any, err := checkBatchWith(
		[]string{"free1", "taken", "free2"},
		false,
		srv.Client(),
		srv.URL+"/api/users/available",
		&buf,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !any {
		t.Error("expected anyAvailable=true")
	}

	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "free1@proton.me") || !strings.HasPrefix(lines[0], "✅") {
		t.Errorf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[1], "taken@proton.me") || !strings.HasPrefix(lines[1], "❌") {
		t.Errorf("line 1: %q", lines[1])
	}
	if !strings.Contains(lines[1], "taken7") || !strings.Contains(lines[1], "taken_dev") {
		t.Errorf("line 1 missing suggestions: %q", lines[1])
	}
	if !strings.Contains(lines[2], "free2@proton.me") || !strings.HasPrefix(lines[2], "✅") {
		t.Errorf("line 2: %q", lines[2])
	}
}

func TestCheckBatch_AllTakenReturnsFalse(t *testing.T) {
	srv := batchServer(t, map[string]availabilityResponse{
		"a": availResp(12106),
		"b": availResp(12106, "b1"),
	})
	defer srv.Close()

	var buf bytes.Buffer
	any, err := checkBatchWith([]string{"a", "b"}, false, srv.Client(), srv.URL+"/api/users/available", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if any {
		t.Error("expected anyAvailable=false when all taken")
	}
}

func TestCheckBatch_JSONOutput(t *testing.T) {
	srv := batchServer(t, map[string]availabilityResponse{
		"free":  availResp(1000),
		"taken": availResp(12106, "taken1"),
	})
	defer srv.Close()

	var buf bytes.Buffer
	any, err := checkBatchWith(
		[]string{"free", "taken"},
		true,
		srv.Client(),
		srv.URL+"/api/users/available",
		&buf,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !any {
		t.Error("expected anyAvailable=true")
	}

	var got []BatchResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Username != "free" || !got[0].Available || got[0].Code != 1000 {
		t.Errorf("result[0] = %+v", got[0])
	}
	if got[1].Username != "taken" || got[1].Available || got[1].Code != 12106 {
		t.Errorf("result[1] = %+v", got[1])
	}
	if len(got[1].Suggestions) != 1 || got[1].Suggestions[0] != "taken1" {
		t.Errorf("result[1].Suggestions = %v", got[1].Suggestions)
	}
}

func TestCheckBatch_ErrorPerNameDoesNotAbortBatch(t *testing.T) {
	// Server returns invalid JSON for the "bad" name; others succeed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("Name")
		if name == "bad" {
			_, _ = w.Write([]byte("not json"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(availResp(1000))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	any, err := checkBatchWith(
		[]string{"ok1", "bad", "ok2"},
		true,
		srv.Client(),
		srv.URL+"/api/users/available",
		&buf,
	)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if !any {
		t.Error("expected anyAvailable=true (ok1 and ok2 available)")
	}

	var got []BatchResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 3 || got[1].Username != "bad" || got[1].Error == "" {
		t.Fatalf("expected middle entry to carry an error, got: %+v", got)
	}
	if !got[0].Available || !got[2].Available {
		t.Errorf("expected ok1 and ok2 available, got: %+v", got)
	}
}

func TestCheckBatch_ConcurrencyCap(t *testing.T) {
	// Track max concurrent handlers observed. Cap must be batchConcurrency.
	var (
		mu               sync.Mutex
		inFlight, maxSeen int
	)
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > maxSeen {
			maxSeen = inFlight
		}
		mu.Unlock()

		// Block until the test releases us.
		<-done

		mu.Lock()
		inFlight--
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(availResp(1000))
	}))
	defer srv.Close()

	names := make([]string, batchConcurrency*3)
	for i := range names {
		names[i] = fmt.Sprintf("u%d", i)
	}

	resultCh := make(chan error, 1)
	var buf bytes.Buffer
	go func() {
		_, err := checkBatchWith(names, true, srv.Client(), srv.URL+"/api/users/available", &buf)
		resultCh <- err
	}()

	// Release all handlers.
	close(done)
	if err := <-resultCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	observed := maxSeen
	mu.Unlock()
	if observed > batchConcurrency {
		t.Errorf("saw %d concurrent handlers, cap is %d", observed, batchConcurrency)
	}
}
