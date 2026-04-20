package signup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		if resp != nil {
			resp.Body.Close()
		}
		return nil, fmt.Errorf("error contacting Proton API: %w", err)
	}
	defer resp.Body.Close()

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

// Check checks username availability and prints the result.
// Returns an error instead of calling os.Exit — let the caller decide.
func Check(username string) error {
	result, err := CheckAvailability(username, defaultHTTPClient)
	if err != nil {
		return fmt.Errorf("checking username: %w", err)
	}

	switch result.Code {
	case 1000:
		fmt.Printf("✅ %s@proton.me is available!\n", username)
	case 12106:
		fmt.Printf("❌ %s@proton.me is already taken.\n", username)
		if len(result.Suggestions) > 0 {
			fmt.Println("\n💡 Suggestions:")
			for _, s := range result.Suggestions {
				fmt.Printf("   • %s@proton.me\n", s)
			}
		}
	default:
		fmt.Printf("⚠️  Unexpected response (code %d)\n", result.Code)
	}
	return nil
}
