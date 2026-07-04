package signin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"proton/internal/log"

	proton "github.com/ProtonMail/go-proton-api"
	"golang.org/x/term"
)

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
func signInWith(opts SigninOptions, p PromptReader, out *os.File) (*SigninResult, error) {
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

	printSummary(out, result)

	// Step 1 explicitly stops after SRP — 2FA + keychain persistence are
	// separate follow-up tasks. Revoking here would force the user to
	// re-authenticate on the next step, so we just drop the client and
	// let the tokens expire naturally.
	return result, nil
}

// printSummary renders the human-facing success message.
func printSummary(w *os.File, r *SigninResult) {
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
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}

	if _, err := SignIn(SigninOptions{}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`proton signin — Sign in to your Proton account (SRP)

Usage:
  proton signin        Prompt for username + password and run the SRP
                       handshake against Proton's servers.

Notes:
  * The password is never sent to Proton; a zero-knowledge proof is
    computed locally and only the proof is transmitted.
  * Two-factor authentication and session persistence are not yet
    implemented — expect the tokens to be discarded at exit.`)
}
