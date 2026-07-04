package signup

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pset builds a lower-cased common-password set from the given words,
// matching what loadCommonPasswords produces.
func pset(words ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[strings.ToLower(w)] = struct{}{}
	}
	return m
}

func hasPenalty(pens []PasswordPenalty, want PasswordPenalty) bool {
	for _, p := range pens {
		if p == want {
			return true
		}
	}
	return false
}

func TestAnalyze_EmptyIsVulnerable(t *testing.T) {
	r := analyzeWith("", pset())
	if r.Score != ScoreVulnerable {
		t.Errorf("score = %q, want %q", r.Score, ScoreVulnerable)
	}
	if !hasPenalty(r.Penalties, PenaltyShort) {
		t.Errorf("expected Short penalty, got %v", r.Penalties)
	}
}

func TestAnalyze_ShortForcesVulnerable(t *testing.T) {
	// 7 chars — below Proton's hard min of 8.
	r := analyzeWith("Ab1!xyz", pset())
	if r.Score != ScoreVulnerable {
		t.Errorf("score = %q, want %q", r.Score, ScoreVulnerable)
	}
	if !hasPenalty(r.Penalties, PenaltyShort) {
		t.Errorf("expected Short penalty, got %v", r.Penalties)
	}
}

func TestAnalyze_CommonPasswordAlwaysVulnerable(t *testing.T) {
	// This password would otherwise be Strong: 12 chars, all classes.
	pw := "Aa1!Aa1!Aa1!"
	r := analyzeWith(pw, pset(pw))
	if r.Score != ScoreVulnerable {
		t.Errorf("common password should force Vulnerable, got %q", r.Score)
	}
	if !hasPenalty(r.Penalties, PenaltyContainsCommonPassword) {
		t.Errorf("expected ContainsCommonPassword, got %v", r.Penalties)
	}
}

func TestAnalyze_CharacterClassPenalties(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		want PasswordPenalty
	}{
		{"missing lowercase", "PASSWORD1!", PenaltyNoLowercase},
		{"missing uppercase", "password1!", PenaltyNoUppercase},
		{"missing number", "Password!!", PenaltyNoNumbers},
		{"missing symbol", "Password11", PenaltyNoSymbols},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := analyzeWith(tc.pw, pset())
			if !hasPenalty(r.Penalties, tc.want) {
				t.Errorf("expected %q in %v", tc.want, r.Penalties)
			}
		})
	}
}

func TestAnalyze_ConsecutiveRunDetected(t *testing.T) {
	r := analyzeWith("Aaaa1234!x", pset())
	if !hasPenalty(r.Penalties, PenaltyConsecutive) {
		t.Errorf("expected Consecutive penalty, got %v", r.Penalties)
	}
}

func TestAnalyze_ProgressiveRunDetected(t *testing.T) {
	// Progressive detection is case-sensitive on code points, so the run
	// must be within one case: "abcd" ascending, "ZYXW" descending.
	cases := []string{"abcd1X!zqp", "ZYXWqrst1!"}
	for _, pw := range cases {
		t.Run(pw, func(t *testing.T) {
			r := analyzeWith(pw, pset())
			if !hasPenalty(r.Penalties, PenaltyProgressive) {
				t.Errorf("expected Progressive penalty, got %v", r.Penalties)
			}
		})
	}
}

func TestAnalyze_StrongPassword(t *testing.T) {
	pw := "Zq7$mVpR!kW9"
	r := analyzeWith(pw, pset())
	if r.Score != ScoreStrong {
		t.Errorf("score = %q, want %q (penalties: %v)", r.Score, ScoreStrong, r.Penalties)
	}
	if len(r.Penalties) != 0 {
		t.Errorf("expected no penalties, got %v", r.Penalties)
	}
}

func TestAnalyze_WeakBucket(t *testing.T) {
	// 12+ chars but missing 2 char classes → Weak.
	r := analyzeWith("longlonglonglong", pset())
	if r.Score != ScoreWeak {
		t.Errorf("score = %q, want %q (penalties: %v)", r.Score, ScoreWeak, r.Penalties)
	}
}

func TestAnalyze_ShortButOtherwiseFineStillVulnerable(t *testing.T) {
	// 9 chars, all classes, no patterns — but still <12 recommended
	// isn't the reason; only Short (<8) forces Vulnerable. 9 chars
	// with 4 classes should score Weak (below recommended length).
	r := analyzeWith("Aa1!bcdef", pset())
	if r.Score != ScoreWeak {
		t.Errorf("score = %q, want %q (penalties: %v)", r.Score, ScoreWeak, r.Penalties)
	}
	if hasPenalty(r.Penalties, PenaltyShort) {
		t.Errorf("did not expect Short for 9-char password: %v", r.Penalties)
	}
}

func TestHasConsecutiveRun(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want bool
	}{
		{"aaa", 3, true},
		{"aab", 3, false},
		{"aabbcc", 3, false},
		{"aaaabbb", 4, true},
		{"", 3, false},
		{"a", 2, false},
	}
	for _, tc := range cases {
		if got := hasConsecutiveRun(tc.s, tc.n); got != tc.want {
			t.Errorf("hasConsecutiveRun(%q, %d) = %v, want %v", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestHasProgressiveRun(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want bool
	}{
		{"abcd", 4, true},
		{"dcba", 4, true},
		{"1234", 4, true},
		{"abce", 4, false},
		{"ab", 3, false},
		{"", 4, false},
		{"abcxyz", 3, true}, // xyz is a run of 3
	}
	for _, tc := range cases {
		if got := hasProgressiveRun(tc.s, tc.n); got != tc.want {
			t.Errorf("hasProgressiveRun(%q, %d) = %v, want %v", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestLoadCommonPasswords(t *testing.T) {
	body := "password\n123456\n\n# a comment\nQwerty\n"
	set := loadCommonPasswords(strings.NewReader(body))
	if len(set) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(set), set)
	}
	if _, ok := set["qwerty"]; !ok {
		t.Errorf("expected case-folded 'qwerty' in set")
	}
	if _, ok := set["# a comment"]; ok {
		t.Errorf("comment line should be skipped")
	}
}

func TestFormatReport_IncludesScoreAndPenalties(t *testing.T) {
	r := PasswordReport{
		Score:     ScoreWeak,
		Length:    10,
		Penalties: []PasswordPenalty{PenaltyNoUppercase, PenaltyNoSymbols},
	}
	var buf bytes.Buffer
	FormatReport(&buf, r)
	out := buf.String()
	for _, want := range []string{"Weak", "10 chars", "uppercase", "symbol"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestFormatReport_NoIssuesWhenStrong(t *testing.T) {
	r := PasswordReport{Score: ScoreStrong, Length: 16}
	var buf bytes.Buffer
	FormatReport(&buf, r)
	if strings.Contains(buf.String(), "Issues:") {
		t.Errorf("did not expect Issues section for strong password: %s", buf.String())
	}
}

func TestValidateFrom_Strong(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	// Random-ish password with all classes and no patterns.
	if err := os.WriteFile(path, []byte("plan: Free\nusername: u\npassword: \"Zq7$mVpR!kW9\"\nrecovery:\n  recovery_email: \"\"\n  recovery_phone: \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	strong, err := validateFrom(path, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strong {
		t.Errorf("expected strong=true, output: %s", buf.String())
	}
}

func TestValidateFrom_WeakJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.yaml")
	if err := os.WriteFile(path, []byte("plan: Free\nusername: u\npassword: \"password\"\nrecovery:\n  recovery_email: \"\"\n  recovery_phone: \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	strong, err := validateFrom(path, true, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strong {
		t.Errorf("expected strong=false for 'password'")
	}
	var got PasswordReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got.Score != ScoreVulnerable {
		t.Errorf("score = %q, want %q", got.Score, ScoreVulnerable)
	}
	if !hasPenalty(got.Penalties, PenaltyContainsCommonPassword) {
		t.Errorf("expected ContainsCommonPassword penalty, got %v", got.Penalties)
	}
}

func TestValidateFrom_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	_, err := validateFrom("/nonexistent/account.yaml", false, &buf)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEmbeddedCommonPasswordsLoads(t *testing.T) {
	set := commonPasswordSet()
	if len(set) < 5000 {
		t.Fatalf("expected embedded wordlist to load thousands of entries, got %d", len(set))
	}
	if _, ok := set["password"]; !ok {
		t.Errorf("expected 'password' in embedded common set")
	}
}
