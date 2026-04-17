package signup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeServer(t *testing.T, code int, errorMsg string, suggestions []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("Name") == "" {
			t.Error("Expected Name query parameter")
		}
		resp := availabilityResponse{
			Code:  code,
			Error: errorMsg,
		}
		resp.Details.Suggestions = suggestions
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCheckAvailability_Available(t *testing.T) {
	server := makeServer(t, 1000, "", nil)
	defer server.Close()

	client := &testClient{server.Client()}

	result, err := CheckAvailabilityWithEndpoint("TestUser", client, server.URL+"/api/users/available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected Available=true")
	}
	if result.Code != 1000 {
		t.Errorf("expected Code=1000, got %d", result.Code)
	}
	if len(result.Suggestions) != 0 {
		t.Errorf("expected no suggestions, got %v", result.Suggestions)
	}
}

func TestCheckAvailability_Taken(t *testing.T) {
	suggestions := []string{"User1", "User2", "User3"}
	server := makeServer(t, 12106, "Username already used", suggestions)
	defer server.Close()

	client := &testClient{server.Client()}

	result, err := CheckAvailabilityWithEndpoint("TakenUser", client, server.URL+"/api/users/available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected Available=false")
	}
	if result.Code != 12106 {
		t.Errorf("expected Code=12106, got %d", result.Code)
	}
	if len(result.Suggestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(result.Suggestions))
	}
	for i, s := range suggestions {
		if result.Suggestions[i] != s {
			t.Errorf("suggestion[%d]: expected %q, got %q", i, s, result.Suggestions[i])
		}
	}
}

func TestCheckAvailability_UnexpectedCode(t *testing.T) {
	server := makeServer(t, 9999, "Something weird", nil)
	defer server.Close()

	client := &testClient{server.Client()}

	result, err := CheckAvailabilityWithEndpoint("User", client, server.URL+"/api/users/available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected Available=false for unexpected code")
	}
	if result.Code != 9999 {
		t.Errorf("expected Code=9999, got %d", result.Code)
	}
}

func TestCheckAvailability_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	client := &testClient{http.DefaultClient}

	_, err := CheckAvailabilityWithEndpoint("User", client, serverURL+"/api/users/available")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestCheckAvailability_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := &testClient{server.Client()}

	_, err := CheckAvailabilityWithEndpoint("User", client, server.URL+"/api/users/available")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCheckAvailability_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &testClient{server.Client()}

	result, err := CheckAvailabilityWithEndpoint("User", client, server.URL+"/api/users/available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Code != 0 {
		t.Errorf("expected Code=0 for empty response, got %d", result.Code)
	}
	if result.Available {
		t.Error("expected Available=false for code 0")
	}
}

// testClient wraps an HTTP client to satisfy the HTTPClient interface in tests.
type testClient struct {
	inner *http.Client
}

func (c *testClient) Do(req *http.Request) (*http.Response, error) {
	return c.inner.Do(req)
}
