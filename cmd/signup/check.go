package signup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"proton/internal/log"
)

// HTTPClient defines the interface for making HTTP requests.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// CheckResult represents the result of a username availability check.
type CheckResult struct {
	Available   bool     `json:"available"`
	Suggestions []string `json:"suggestions"`
	Code        int      `json:"code"`
}

type availabilityResponse struct {
	Code    int    `json:"Code"`
	Error   string `json:"Error"`
	Details struct {
		Suggestions []string `json:"Suggestions"`
	} `json:"Details"`
}

// ProtonAvailabilityEndpoint is the default Proton API endpoint for username checks.
const ProtonAvailabilityEndpoint = "https://mail-api.proton.me/api/users/available"

// CheckAvailability checks username availability against the Proton API.
func CheckAvailability(username string, client HTTPClient) (*CheckResult, error) {
	return CheckAvailabilityWithEndpoint(username, client, ProtonAvailabilityEndpoint)
}

// CheckAvailabilityWithEndpoint checks username availability against a given API endpoint.
// This function is testable and does not perform I/O to stdout/stderr or call os.Exit.
func CheckAvailabilityWithEndpoint(username string, client HTTPClient, endpoint string) (*CheckResult, error) {
	log.Debug("Checking username availability", "username", username)

	endpoint = endpoint + "?Name=" + url.QueryEscape(username)
	log.Debug("Making API request", "endpoint", endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("x-pm-appversion", "Other")
	req.Header.Set("x-pm-apiversion", "3")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Per net/http.Client.Do: on error, resp is nil. Nothing to close.
		return nil, fmt.Errorf("error contacting Proton API: %w", err)
	}
	defer func() {
		// Body.Close on a fully-read response should never fail; log at
		// debug level if it ever does so we notice in --verbose runs.
		if cerr := resp.Body.Close(); cerr != nil {
			log.Debug("closing response body", "error", cerr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var apiResult availabilityResponse
	if err := json.Unmarshal(body, &apiResult); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	log.Debug("API response received", "code", apiResult.Code, "error", apiResult.Error)

	result := &CheckResult{
		Code:        apiResult.Code,
		Suggestions: apiResult.Details.Suggestions,
	}

	switch apiResult.Code {
	case 1000:
		result.Available = true
		log.Info("Username is available", "username", username)
	case 12106:
		result.Available = false
		log.Info("Username is taken", "username", username, "suggestions_count", len(apiResult.Details.Suggestions))
	default:
		result.Available = false
		log.Warn("Unexpected API response", "code", apiResult.Code, "error", apiResult.Error)
	}

	return result, nil
}

// defaultHTTPClient is a pre-configured HTTP client with sensible timeouts.
var defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Check checks a single username's availability and prints the result.
// Thin wrapper around CheckBatch for backward compatibility.
// Returns an error instead of calling os.Exit — let the caller decide.
func Check(username string) error {
	_, err := CheckBatch([]string{username}, false)
	return err
}

// BatchResult is one entry in a CheckBatch response, ready for human or JSON output.
type BatchResult struct {
	Username    string   `json:"username"`
	Available   bool     `json:"available"`
	Code        int      `json:"code,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// batchConcurrency caps in-flight requests to avoid hammering the API.
const batchConcurrency = 5

// CheckBatch checks many usernames concurrently, prints the results, and
// reports whether at least one was available.
//
// When jsonOut is true, results are emitted as a JSON array on stdout with no
// other decoration — safe to pipe. Otherwise the emoji/human format is used.
// Input order is preserved in both output modes.
func CheckBatch(usernames []string, jsonOut bool) (anyAvailable bool, err error) {
	return checkBatchWith(usernames, jsonOut, defaultHTTPClient, ProtonAvailabilityEndpoint, os.Stdout)
}

// checkBatchWith is the injectable core of CheckBatch: it takes the HTTP
// client, endpoint, and output writer so tests don't need to touch the network
// or stdout.
func checkBatchWith(usernames []string, jsonOut bool, client HTTPClient, endpoint string, out io.Writer) (bool, error) {
	if len(usernames) == 0 {
		return false, fmt.Errorf("no usernames provided")
	}

	results := make([]BatchResult, len(usernames))
	sem := make(chan struct{}, batchConcurrency)
	var wg sync.WaitGroup

	for i, name := range usernames {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, name string) {
			defer wg.Done()
			defer func() { <-sem }()

			res, err := CheckAvailabilityWithEndpoint(name, client, endpoint)
			if err != nil {
				results[i] = BatchResult{Username: name, Error: err.Error()}
				return
			}
			results[i] = BatchResult{
				Username:    name,
				Available:   res.Available,
				Code:        res.Code,
				Suggestions: res.Suggestions,
			}
		}(i, name)
	}
	wg.Wait()

	any := false
	for _, r := range results {
		if r.Available {
			any = true
			break
		}
	}

	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return any, fmt.Errorf("encoding json: %w", err)
		}
		return any, nil
	}

	printBatchHuman(out, results)
	return any, nil
}

// printBatchHuman writes the emoji/human-friendly output for a batch result.
func printBatchHuman(out io.Writer, results []BatchResult) {
	for _, r := range results {
		switch {
		case r.Error != "":
			fmt.Fprintf(out, "⚠️  %s@proton.me — error: %s\n", r.Username, r.Error)
		case r.Available:
			fmt.Fprintf(out, "✅ %s@proton.me\n", r.Username)
		case r.Code == 12106:
			if len(r.Suggestions) > 0 {
				fmt.Fprintf(out, "❌ %s@proton.me      (suggestions: %s)\n", r.Username, strings.Join(r.Suggestions, ", "))
			} else {
				fmt.Fprintf(out, "❌ %s@proton.me\n", r.Username)
			}
		default:
			fmt.Fprintf(out, "⚠️  %s@proton.me — unexpected response (code %d)\n", r.Username, r.Code)
		}
	}
}
