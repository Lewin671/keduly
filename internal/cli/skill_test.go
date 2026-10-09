package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillInstall(t *testing.T) {
	h := newHarness(t, true)
	source, err := os.ReadFile("../../skills/keduly/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	// The default is the skills directory agents share, and it needs no login.
	home := t.TempDir()
	h.env = map[string]string{"HOME": home, "XDG_CONFIG_HOME": t.TempDir()}
	target := filepath.Join(home, ".agents", "skills", "keduly")
	wantIn(t, h.ok("skill", "install"), target)
	if got, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(got) != string(source) {
		t.Fatalf("the installed skill differs from skills/keduly/SKILL.md")
	}

	// Installing again replaces what is there, stale files included.
	stale := filepath.Join(target, "stale.md")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.ok("skill", "install")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("a stale file survived: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(got) != string(source) {
		t.Fatalf("the skill was not replaced")
	}

	// Claude Code reads its own directory.
	wantIn(t, h.ok("skill", "install", "--claude"), filepath.Join(home, ".claude", "skills", "keduly"))
	h.env["CLAUDE_CONFIG_DIR"] = filepath.Join(home, "elsewhere")
	wantIn(t, h.ok("skill", "install", "--claude"), filepath.Join(home, "elsewhere", "skills", "keduly"))
	wantIn(t, h.ok("skill", "install"), target)

	// --project installs into the current directory.
	t.Chdir(t.TempDir())
	h.ok("skill", "install", "--project")
	h.ok("skill", "install", "--project", "--claude")
	for _, dir := range []string{".agents", ".claude"} {
		if _, err := os.Stat(filepath.Join(dir, "skills", "keduly", "SKILL.md")); err != nil {
			t.Fatal(err)
		}
	}
	h.fails(2, []string{"skill", "install", "--claude", "--dir", "x"}, "cannot be combined")

	// --dir names another agent's skills directory.
	other := t.TempDir()
	wantIn(t, h.ok("skill", "install", "--dir", other, "--json"), `"installed":true`)
	if _, err := os.Stat(filepath.Join(other, "keduly", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	// A symbolic link is a working copy and stays untouched.
	linked, work := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "SKILL.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(work, filepath.Join(linked, "keduly")); err != nil {
		t.Fatal(err)
	}
	out := h.ok("skill", "install", "--dir", linked, "--json")
	if !strings.Contains(out, `"installed":false`) {
		t.Fatalf("a link must be left alone: %s", out)
	}
	if got, _ := os.ReadFile(filepath.Join(work, "SKILL.md")); string(got) != "mine" {
		t.Fatalf("the working copy changed: %s", got)
	}
}
