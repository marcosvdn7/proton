package signup

import (
	_ "embed"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
)

// PasswordScore mirrors the enum used by Proton's own client-side
// analyser (see WebClients `passwordStrengthIndicator/interface.ts`,
// backed by `@protontech/pass-rust-core/password`). Three buckets:
// Vulnerable / Weak / Strong.
type PasswordScore string

const (
	ScoreVulnerable PasswordScore = "Vulnerable"
	ScoreWeak       PasswordScore = "Weak"
	ScoreStrong     PasswordScore = "Strong"
)

// PasswordPenalty is one of the specific reasons a password lost
// points. Names match the Proton enum so future migration to
// pass-rust-core (if it goes public) is a straight swap.
type PasswordPenalty string

const (
	PenaltyShort                  PasswordPenalty = "Short"
	PenaltyNoLowercase            PasswordPenalty = "NoLowercase"
	PenaltyNoUppercase            PasswordPenalty = "NoUppercase"
	PenaltyNoNumbers              PasswordPenalty = "NoNumbers"
	PenaltyNoSymbols              PasswordPenalty = "NoSymbols"
	PenaltyConsecutive            PasswordPenalty = "Consecutive"
	PenaltyProgressive            PasswordPenalty = "Progressive"
	PenaltyContainsCommonPassword PasswordPenalty = "ContainsCommonPassword"
)

// MinPasswordLength is Proton's hard minimum (see protoncore_android
// pass-validator: `private const val MIN_PASSWORD_LENGTH = 8`).
const MinPasswordLength = 8

// RecommendedPasswordLength is the length above which a password
// stops being penalised for `Short`. Chosen to match Proton's blog
// recommendation of "at least 12 characters".
const RecommendedPasswordLength = 12

// PasswordReport is the structured result of Analyze.
type PasswordReport struct {
	Score     PasswordScore     `json:"score"`
	Penalties []PasswordPenalty `json:"penalties,omitempty"`
	Length    int               `json:"length"`
}

//go:embed data/common_passwords.txt
var commonPasswordsRaw string

var (
	commonPasswordsOnce sync.Once
	commonPasswords     map[string]struct{}
)

// commonPasswordSet lazily builds the lookup set from the embedded
// wordlist. Rows are trimmed and lowercased for a case-insensitive
// match.
func commonPasswordSet() map[string]struct{} {
	commonPasswordsOnce.Do(func() {
		commonPasswords = loadCommonPasswords(strings.NewReader(commonPasswordsRaw))
	})
	return commonPasswords
}

// loadCommonPasswords reads a newline-separated wordlist into a set.
// Exported-shape so tests can feed a small inline list instead of the
// embedded 10k rows.
func loadCommonPasswords(r io.Reader) map[string]struct{} {
	set := make(map[string]struct{}, 10000)
	sc := bufio.NewScanner(r)
	// Some entries in the SecLists file may be longer than the default
	// 64k line limit if the file gets updated; bump it defensively.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
}

// Analyze runs Proton's model on the given password and returns a
// structured report. The scoring is intentionally close to what the
// Proton web client shows on account.proton.me, so users don't get
// contradictory feedback.
func Analyze(password string) PasswordReport {
	return analyzeWith(password, commonPasswordSet())
}

// analyzeWith is the pure, injectable core of Analyze. Tests pass in
// a small common-password set instead of loading the embedded file.
func analyzeWith(password string, common map[string]struct{}) PasswordReport {
	report := PasswordReport{Length: len(password)}

	if password == "" {
		report.Score = ScoreVulnerable
		report.Penalties = append(report.Penalties, PenaltyShort)
		return report
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r):
			hasSymbol = true
		}
	}

	if len(password) < MinPasswordLength {
		report.Penalties = append(report.Penalties, PenaltyShort)
	}
	if !hasLower {
		report.Penalties = append(report.Penalties, PenaltyNoLowercase)
	}
	if !hasUpper {
		report.Penalties = append(report.Penalties, PenaltyNoUppercase)
	}
	if !hasDigit {
		report.Penalties = append(report.Penalties, PenaltyNoNumbers)
	}
	if !hasSymbol {
		report.Penalties = append(report.Penalties, PenaltyNoSymbols)
	}
	if hasConsecutiveRun(password, 3) {
		report.Penalties = append(report.Penalties, PenaltyConsecutive)
	}
	if hasProgressiveRun(password, 4) {
		report.Penalties = append(report.Penalties, PenaltyProgressive)
	}
	if _, ok := common[strings.ToLower(password)]; ok {
		report.Penalties = append(report.Penalties, PenaltyContainsCommonPassword)
	}

	report.Score = scoreFrom(len(password), report.Penalties)
	return report
}

// hasConsecutiveRun reports whether the password contains a run of
// `n` identical characters in a row (e.g. "aaaa"). It's Proton's
// `Consecutive` penalty.
func hasConsecutiveRun(s string, n int) bool {
	if n <= 1 {
		return false
	}
	runes := []rune(s)
	run := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			run++
			if run >= n {
				return true
			}
		} else {
			run = 1
		}
	}
	return false
}

// hasProgressiveRun reports whether the password contains an
// ascending or descending sequence of `n` consecutive code points
// (e.g. "1234", "abcd", "wxyz", "dcba"). It's Proton's `Progressive`
// penalty.
func hasProgressiveRun(s string, n int) bool {
	if n <= 1 {
		return false
	}
	runes := []rune(s)
	if len(runes) < n {
		return false
	}
	asc, desc := 1, 1
	for i := 1; i < len(runes); i++ {
		switch runes[i] - runes[i-1] {
		case 1:
			asc++
			desc = 1
		case -1:
			desc++
			asc = 1
		default:
			asc, desc = 1, 1
		}
		if asc >= n || desc >= n {
			return true
		}
	}
	return false
}

// scoreFrom picks a bucket. The thresholds are tuned to match Proton
// web's rough behaviour: a password with a common blocklist hit or
// under the hard minimum is always Vulnerable; otherwise Weak when
// there are meaningful gaps, Strong only when length + charset checks
// clear.
func scoreFrom(length int, penalties []PasswordPenalty) PasswordScore {
	has := func(p PasswordPenalty) bool {
		for _, x := range penalties {
			if x == p {
				return true
			}
		}
		return false
	}

	// Any of these force Vulnerable regardless of the rest.
	if has(PenaltyShort) || has(PenaltyContainsCommonPassword) {
		return ScoreVulnerable
	}

	// A structural weakness (short-ish, missing multiple classes, or an
	// obvious pattern) drops to Weak.
	classGaps := 0
	if has(PenaltyNoLowercase) {
		classGaps++
	}
	if has(PenaltyNoUppercase) {
		classGaps++
	}
	if has(PenaltyNoNumbers) {
		classGaps++
	}
	if has(PenaltyNoSymbols) {
		classGaps++
	}

	switch {
	case has(PenaltyConsecutive) || has(PenaltyProgressive):
		return ScoreWeak
	case classGaps >= 2:
		return ScoreWeak
	case length < RecommendedPasswordLength:
		return ScoreWeak
	default:
		return ScoreStrong
	}
}

// FormatReport renders a report in human-readable form (used by
// `signup validate` and `signup fill`).
func FormatReport(w io.Writer, r PasswordReport) {
	icon := "✅"
	switch r.Score {
	case ScoreVulnerable:
		icon = "❌"
	case ScoreWeak:
		icon = "⚠️ "
	}
	fmt.Fprintf(w, "%s Password strength: %s (%d chars)\n", icon, r.Score, r.Length)
	if len(r.Penalties) == 0 {
		return
	}
	fmt.Fprintln(w, "Issues:")
	for _, p := range r.Penalties {
		fmt.Fprintf(w, "  • %s\n", penaltyExplanation(p))
	}
}

// Validate loads account.yaml, analyses its password, prints a report,
// and returns whether the score is Strong. Exposed as a subcommand:
// `proton signup validate`.
func Validate(jsonOut bool) (bool, error) {
	return validateFrom(DefaultConfigPath, jsonOut, os.Stdout)
}

// validateFrom is the injectable core of Validate: takes the config
// path and output writer so tests don't touch the real filesystem or
// stdout.
func validateFrom(path string, jsonOut bool, out io.Writer) (bool, error) {
	cfg, err := loadConfigFrom(path)
	if err != nil {
		return false, err
	}
	report := Analyze(cfg.Password)
	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return report.Score == ScoreStrong, fmt.Errorf("encoding json: %w", err)
		}
		return report.Score == ScoreStrong, nil
	}
	FormatReport(out, report)
	return report.Score == ScoreStrong, nil
}

// penaltyExplanation maps the Proton-style enum to a short user-facing
// sentence. Kept in one place so localisation later touches only this.
func penaltyExplanation(p PasswordPenalty) string {
	switch p {
	case PenaltyShort:
		return fmt.Sprintf("too short (want at least %d characters)", MinPasswordLength)
	case PenaltyNoLowercase:
		return "missing lowercase letter"
	case PenaltyNoUppercase:
		return "missing uppercase letter"
	case PenaltyNoNumbers:
		return "missing number"
	case PenaltyNoSymbols:
		return "missing symbol"
	case PenaltyConsecutive:
		return "contains a repeated character run (e.g. 'aaaa')"
	case PenaltyProgressive:
		return "contains a sequential run (e.g. '1234' or 'abcd')"
	case PenaltyContainsCommonPassword:
		return "appears on a well-known common-passwords list"
	default:
		return string(p)
	}
}
