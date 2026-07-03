package signup

import (
	"errors"
	"testing"
)

// fakeEnv returns an EnvFunc backed by a map.
func fakeEnv(m map[string]string) EnvFunc {
	return func(k string) string { return m[k] }
}

// fakeLookPath returns a LookPathFunc that succeeds only for names in found.
// The returned path is the name itself, matching exec.LookPath for tools on PATH.
func fakeLookPath(found ...string) LookPathFunc {
	set := make(map[string]struct{}, len(found))
	for _, n := range found {
		set[n] = struct{}{}
	}
	return func(name string) (string, error) {
		if _, ok := set[name]; ok {
			return name, nil
		}
		return "", errors.New("not found: " + name)
	}
}

// asExec asserts the Clipboard is an *execClipboard with the given cmd and args.
func asExec(t *testing.T, c Clipboard, wantCmd string, wantArgs ...string) {
	t.Helper()
	ec, ok := c.(*execClipboard)
	if !ok {
		t.Fatalf("expected *execClipboard, got %T", c)
	}
	if ec.cmd != wantCmd {
		t.Errorf("cmd = %q, want %q", ec.cmd, wantCmd)
	}
	if len(ec.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", ec.args, wantArgs)
	}
	for i, a := range wantArgs {
		if ec.args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, ec.args[i], a)
		}
	}
}

func TestPickClipboard_Darwin(t *testing.T) {
	c, err := pickClipboard("darwin", fakeEnv(nil), fakeLookPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "pbcopy")
}

func TestPickClipboard_Windows(t *testing.T) {
	c, err := pickClipboard("windows", fakeEnv(nil), fakeLookPath())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "clip")
}

func TestPickClipboard_Unsupported(t *testing.T) {
	_, err := pickClipboard("plan9", fakeEnv(nil), fakeLookPath())
	if err == nil {
		t.Fatal("expected error for unsupported GOOS")
	}
}

func TestPickLinuxClipboard_WSLPrefersClipExe(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(map[string]string{"WSL_DISTRO_NAME": "Ubuntu", "WAYLAND_DISPLAY": "wayland-0"}),
		fakeLookPath("clip.exe", "wl-copy", "xclip"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "clip.exe")
}

func TestPickLinuxClipboard_WSLFallsThroughWhenClipExeMissing(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(map[string]string{"WSL_DISTRO_NAME": "Ubuntu", "WAYLAND_DISPLAY": "wayland-0"}),
		fakeLookPath("wl-copy"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "wl-copy")
}

func TestPickLinuxClipboard_WaylandPrefersWlCopy(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}),
		fakeLookPath("wl-copy", "xclip"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "wl-copy")
}

func TestPickLinuxClipboard_WaylandFallsBackToX11WhenWlCopyMissing(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(map[string]string{"WAYLAND_DISPLAY": "wayland-0"}),
		fakeLookPath("xclip"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "xclip", "-selection", "clipboard")
}

func TestPickLinuxClipboard_X11PrefersXclip(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(nil),
		fakeLookPath("xclip", "xsel"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "xclip", "-selection", "clipboard")
}

func TestPickLinuxClipboard_X11FallsBackToXsel(t *testing.T) {
	c, err := pickLinuxClipboard(
		fakeEnv(nil),
		fakeLookPath("xsel"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asExec(t, c, "xsel", "--clipboard", "--input")
}

func TestPickLinuxClipboard_NoToolFound(t *testing.T) {
	_, err := pickLinuxClipboard(fakeEnv(nil), fakeLookPath())
	if err == nil {
		t.Fatal("expected error when no clipboard tool available")
	}
}
