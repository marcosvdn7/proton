package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

type availabilityResponse struct {
	Code    int    `json:"Code"`
	Error   string `json:"Error"`
	Details struct {
		Suggestions []string `json:"Suggestions"`
	} `json:"Details"`
}

func Check(username string) {
	endpoint := "https://mail-api.proton.me/api/users/available?Name=" + url.QueryEscape(username)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("x-pm-appversion", "Other")
	req.Header.Set("x-pm-apiversion", "3")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error contacting Proton API: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	var result availabilityResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	switch result.Code {
	case 1000:
		fmt.Printf("✅ %s@proton.me is available!\n", username)
	case 12106:
		fmt.Printf("❌ %s@proton.me is already taken.\n", username)
		if len(result.Details.Suggestions) > 0 {
			fmt.Println("\n💡 Suggestions:")
			for _, s := range result.Details.Suggestions {
				fmt.Printf("   • %s@proton.me\n", s)
			}
		}
	default:
		fmt.Printf("⚠️  Unexpected response (code %d): %s\n", result.Code, result.Error)
	}
}
