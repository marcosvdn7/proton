package signin

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"proton/internal/keychain"
	"proton/internal/log"

	proton "github.com/ProtonMail/go-proton-api"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// usernameFromAccountYAML reads the same account.yaml layout the signup
// helper writes. Only the Username field is needed here — anything else in
// that file is out of scope for signin.
func usernameFromAccountYAML(path string) (string, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cfg struct {
		Username string `yaml:"username"`
	}
	if err := yaml.Unmarshal(blob, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	return strings.TrimSpace(cfg.Username), nil
}

// appVersion is the value sent in the x-pm-appversion header that Proton
// requires on every request. The stable form is <product>@<semver>; picking a
// clearly non-official prefix keeps our traffic distinguishable from the
// official web/desktop clients in Proton's abuse metrics.
const appVersion = "proton-cli@0.1.0"

// defaultTimeout bounds a full SRP handshake (AuthInfo + Auth). Real handshakes
// take ~1-2s on a healthy link, so 30s is a very generous ceiling that still
// gives up before a hung TCP connection wastes the user's terminal.
const defaultTimeout = 30 * time.Second

// SigninResult is returned by SignIn after a successful SRP handshake.
// It intentionally does NOT include the raw password (already wiped) and
// does not include the tokens by default — the caller decides how to
// display or persist them. Currently we only print UserID + basic flags,
// but the tokens are here so a future persistence step can grab them
// without touching signin's internals.
type SigninResult struct {
	UserID         string
	UID            string
	AccessToken    string
	RefreshToken   string
	Scope          string
	Requires2FA    bool
	TwoPasswordMod bool
}

// SigninOptions is the configurable surface of SignIn. Everything here has a
// sensible default so callers can pass a zero value in tests.
type SigninOptions struct {
	// HostURL overrides the Proton API base URL. Empty means production.
	HostURL string
	// AppVersion overrides the x-pm-appversion header. Empty means the
	// package-level appVersion constant.
	AppVersion string
	// Timeout bounds the whole handshake. Zero means defaultTimeout.
	Timeout time.Duration
	// Transport overrides the HTTP transport used by go-proton-api. This
	// exists so tests can plug in proton.InsecureTransport() to talk to
	// the bundled in-memory server (self-signed TLS). Production callers
	// leave this nil.
	Transport http.RoundTripper
	// Persister is the keychain seam. Nil means the real OS keychain via
	// the internal/keychain package. Tests inject a fake so the CI
	// keyring is not touched.
	Persister SessionPersister
}

// SessionPersister is the narrow interface signin needs from the keychain
// layer: save the current session and forget it on demand. Only these two
// verbs are used at sign-in time; --status / --logout live in Run and can
// call keychain.Load / keychain.Delete directly.
type SessionPersister interface {
	Save(username string, s keychain.Session) error
}

// realKeychain is the production SessionPersister — thin wrapper delegating
// to the internal/keychain package.
type realKeychain struct{}

func (realKeychain) Save(username string, s keychain.Session) error {
	return keychain.Save(username, s)
}

// PromptReader is the injectable input surface for SignIn, so tests can feed
// canned username/password without a real terminal.
type PromptReader interface {
	ReadUsername(prompt string) (string, error)
	ReadPassword(prompt string) ([]byte, error)
}

// SignIn runs the SRP login handshake with prompts on stderr/stdin and prints a
// short result summary. Returns the SigninResult on success.
func SignIn(opts SigninOptions) (*SigninResult, error) {
	return signInWith(opts, &ttyPrompt{in: os.Stdin, out: os.Stderr}, os.Stdout)
}

// signInWith is the injectable core: takes a prompter and an output writer so
// tests never touch the real terminal or stdout.
func signInWith(opts SigninOptions, p PromptReader, out io.Writer) (*SigninResult, error) {
	username, err := p.ReadUsername("Proton username or email: ")
	if err != nil {
		return nil, fmt.Errorf("reading username: %w", err)
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}

	password, err := p.ReadPassword("Password: ")
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	// Zero the password buffer as soon as go-proton-api is done with it,
	// even on error paths. defer runs before we return the SigninResult,
	// so it never leaves this function alive.
	defer wipe(password)
	if len(password) == 0 {
		return nil, errors.New("password cannot be empty")
	}

	if opts.AppVersion == "" {
		opts.AppVersion = appVersion
	}
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}

	managerOpts := []proton.Option{
		proton.WithAppVersion(opts.AppVersion),
	}
	if opts.HostURL != "" {
		managerOpts = append(managerOpts, proton.WithHostURL(opts.HostURL))
	}
	if opts.Transport != nil {
		managerOpts = append(managerOpts, proton.WithTransport(opts.Transport))
	}

	m := proton.New(managerOpts...)
	defer m.Close()

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	log.Debug("Starting SRP login", "username", username, "host", opts.HostURL, "appVersion", opts.AppVersion)

	client, auth, err := m.NewClientWithLogin(ctx, username, password)
	if err != nil {
		return nil, translateAuthError(err)
	}
	defer client.Close()

	log.Info("Signed in", "userID", auth.UserID, "twoFA", auth.TwoFA.Enabled, "passwordMode", auth.PasswordMode)

	result := &SigninResult{
		UserID:         auth.UserID,
		UID:            auth.UID,
		AccessToken:    auth.AccessToken,
		RefreshToken:   auth.RefreshToken,
		Scope:          auth.Scope,
		Requires2FA:    auth.TwoFA.Enabled&proton.HasTOTP != 0,
		TwoPasswordMod: auth.PasswordMode == proton.TwoPasswordMode,
	}

	// Persist the fresh session so subsequent CLI invocations reuse it
	// instead of re-prompting for the password. If the account has 2FA or
	// two-password mode we still save — the tokens are already valid for
	// unscoped calls, and the follow-up 2FA step will overwrite this
	// bundle on success.
	persister := opts.Persister
	if persister == nil {
		persister = realKeychain{}
	}
	session := keychain.Session{
		UID:          auth.UID,
		AccessToken:  auth.AccessToken,
		RefreshToken: auth.RefreshToken,
		Scope:        auth.Scope,
	}
	if err := persister.Save(username, session); err != nil {
		// Persistence failure is not a hard sign-in failure — the user
		// has a valid session in this process. Warn and continue so
		// they at least see the success message and can act on the
		// warning (fix keychain, re-run signin).
		log.Warn("Could not persist session to keychain", "err", err)
		fmt.Fprintf(out, "⚠️  Sign-in succeeded but session could not be saved: %v\n", err)
	} else {
		log.Info("Session persisted to keychain", "user", username)
	}

	// Refresh events must also persist, otherwise the RefreshToken saved
	// above becomes stale and the next invocation gets a 401 loop.
	// go-proton-api rotates tokens on /auth/v4/refresh and fires this
	// handler with the new pair.
	client.AddAuthHandler(func(refreshed proton.Auth) {
		refreshedSession := keychain.Session{
			UID:          refreshed.UID,
			AccessToken:  refreshed.AccessToken,
			RefreshToken: refreshed.RefreshToken,
			Scope:        refreshed.Scope,
		}
		if err := persister.Save(username, refreshedSession); err != nil {
			log.Warn("Could not persist refreshed session", "err", err)
		}
	})

	printSummary(out, result)

	// SRP + persistence done. 2FA + two-password unlock remain as
	// follow-up work; see docs/AGENT.md for scope. We deliberately do
	// NOT call client.AuthDelete here — that would revoke the session we
	// just saved.
	return result, nil
}

// printSummary renders the human-facing success message.
func printSummary(w io.Writer, r *SigninResult) {
	fmt.Fprintln(w, "✅ SRP handshake succeeded")
	fmt.Fprintf(w, "   User ID:      %s\n", r.UserID)
	fmt.Fprintf(w, "   Scope:        %s\n", r.Scope)
	if r.Requires2FA {
		fmt.Fprintln(w, "⚠️  Two-factor authentication is enabled on this account.")
		fmt.Fprintln(w, "   TOTP support is not yet implemented — you can only complete")
		fmt.Fprintln(w, "   sign-in in a Proton web client for now.")
	}
	if r.TwoPasswordMod {
		fmt.Fprintln(w, "⚠️  Account uses two-password mode. The mailbox password is needed")
		fmt.Fprintln(w, "   to unlock keys and is not yet handled by this CLI.")
	}
}

// translateAuthError maps low-level lib errors to more actionable messages
// without leaking anything about the password.
func translateAuthError(err error) error {
	if errors.Is(err, proton.ErrInvalidProof) {
		return errors.New("server proof did not verify — refusing to trust this response")
	}
	// go-proton-api wraps HTTP 4xx/5xx in resty.ResponseError; we don't need
	// to peel that here, the string form is already actionable.
	return fmt.Errorf("sign-in failed: %w", err)
}

// wipe zeroes a password buffer in place. Best-effort — Go strings from the
// runtime's stdin buffer may still linger elsewhere, but the []byte we hand
// to go-proton-api is scrubbed.
func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ttyPrompt is the production PromptReader. Username reads a plain line from
// stdin, password reads with echo disabled via x/term.
type ttyPrompt struct {
	in  *os.File
	out *os.File
}

func (p *ttyPrompt) ReadUsername(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	r := bufio.NewReader(p.in)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (p *ttyPrompt) ReadPassword(prompt string) ([]byte, error) {
	fmt.Fprint(p.out, prompt)
	// x/term needs the underlying FD to disable echo. If stdin isn't a real
	// terminal (piped input in scripts, CI, tests) fall back to a plain
	// bufio read — the caller has already decided that's OK.
	fd := int(p.in.Fd())
	if !term.IsTerminal(fd) {
		r := bufio.NewReader(p.in)
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return []byte(strings.TrimRight(line, "\r\n")), nil
	}
	pw, err := term.ReadPassword(fd)
	// Always emit a newline so the next output starts on its own line —
	// term.ReadPassword swallows the Enter keystroke.
	fmt.Fprintln(p.out)
	return pw, err
}

// Run handles the signin subcommand.
//
//	proton signin               — prompt for creds, do SRP, save session
//	proton signin --status      — show saved-session summary (never prints tokens)
//	proton signin --logout      — wipe saved session for the account
//	proton signin help          — usage
func Run(args []string) {
	fs := flag.NewFlagSet("signin", flag.ExitOnError)
	var (
		status   bool
		logout   bool
		username string
	)
	fs.BoolVar(&status, "status", false, "show saved-session summary (does not print tokens)")
	fs.BoolVar(&logout, "logout", false, "delete saved session from the OS keychain")
	fs.StringVar(&username, "user", "", "target Proton username (defaults to account.yaml)")
	fs.Usage = printUsage

	// Preserve the existing 'signin help' subcommand form so we don't
	// break shell muscle memory.
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	if status && logout {
		fmt.Fprintln(os.Stderr, "Error: --status and --logout are mutually exclusive")
		os.Exit(2)
	}

	switch {
	case status:
		if err := runStatus(username, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case logout:
		if err := runLogout(username, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		if _, err := SignIn(SigninOptions{}); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// resolveUsername returns the username to operate on for --status / --logout.
// Explicit --user wins; otherwise fall back to account.yaml so users don't
// have to re-type their address every command.
func resolveUsername(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	u, err := usernameFromAccountYAML("account.yaml")
	if err != nil {
		return "", fmt.Errorf("no --user given and could not read account.yaml: %w", err)
	}
	if u == "" {
		return "", errors.New("no --user given and account.yaml has no username")
	}
	return u, nil
}

func runStatus(flagUser string, out io.Writer) error {
	user, err := resolveUsername(flagUser)
	if err != nil {
		return err
	}
	s, err := keychain.Load(user)
	if errors.Is(err, keychain.ErrNotFound) {
		fmt.Fprintf(out, "No saved session for %s. Run `proton signin`.\n", user)
		return nil
	}
	if err != nil {
		return err
	}
	// Never print the tokens themselves — not even truncated. If a user
	// really wants to debug them they can query the OS keychain directly.
	fmt.Fprintf(out, "Saved session for %s\n", user)
	fmt.Fprintf(out, "  UID:         %s\n", s.UID)
	fmt.Fprintf(out, "  Scope:       %s\n", s.Scope)
	fmt.Fprintf(out, "  Saved at:    %s\n", s.SavedAt.Format(time.RFC3339))
	return nil
}

func runLogout(flagUser string, out io.Writer) error {
	user, err := resolveUsername(flagUser)
	if err != nil {
		return err
	}
	if err := keychain.Delete(user); err != nil {
		return err
	}
	fmt.Fprintf(out, "Signed out %s.\n", user)
	return nil
}

func printUsage() {
	fmt.Println(`proton signin — Sign in to your Proton account (SRP)

Usage:
  proton signin                Prompt for username + password and run the SRP
                               handshake against Proton's servers. On success
                               the session tokens are saved to the OS keychain.
  proton signin --status       Show the saved session summary for the current
                               account (UID + scope + save time). Tokens are
                               never printed.
  proton signin --logout       Delete the saved session from the OS keychain.
  proton signin --user <name>  Override the account resolved from account.yaml,
                               used together with --status / --logout.

Notes:
  * The password is never sent to Proton; a zero-knowledge proof is
    computed locally and only the proof is transmitted.
  * Session tokens are stored in the OS keychain (macOS Keychain,
    Windows Credential Manager, Linux Secret Service).
  * Two-factor authentication and two-password mailbox unlock are not
    yet implemented.`)
}
