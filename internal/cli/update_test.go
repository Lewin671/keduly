package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Lewin671/keduly/internal/cli"
)

// releases serves a fake GitHub release page and its files.
type releases struct {
	latest    string
	badSum    bool
	mu        sync.Mutex
	downloads []string
}

// binary is what the archive of a release holds.
func binary(tag string) string { return "#!/bin/sh\necho keduly " + tag + "\n" }

func (r *releases) archive(tag string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct{ name, body string }{{"LICENSE", "MIT"}, {"keduly", binary(tag)}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(f.body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func (r *releases) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := fmt.Sprintf("keduly-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if req.URL.Path == "/releases/latest" {
		http.Redirect(w, req, "/releases/tag/"+r.latest, http.StatusFound)
		return
	}
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 5 || parts[1] != "releases" || parts[2] != "download" {
		http.NotFound(w, req)
		return
	}
	tag, file := parts[3], parts[4]
	r.mu.Lock()
	r.downloads = append(r.downloads, tag+"/"+file)
	r.mu.Unlock()
	data := r.archive(tag)
	switch file {
	case name:
		w.Write(data)
	case "checksums.txt":
		if r.badSum {
			data = []byte("tampered")
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(w, "%s  keduly-other-arch.tar.gz\n%s *%s\n", strings.Repeat("0", 64), hex.EncodeToString(sum[:]), name)
	default:
		http.NotFound(w, req)
	}
}

// updater is an installed keduly at version, reached through a link as on a PATH.
type updater struct {
	t       *testing.T
	r       *releases
	url     string
	version string
	dir     string
	link    string
}

func newUpdater(t *testing.T, version, latest string) *updater {
	t.Helper()
	r := &releases{latest: latest}
	s := httptest.NewServer(r)
	t.Cleanup(s.Close)
	// The update reports the file it replaced, links resolved; the temporary directory may be one.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	u := &updater{t: t, r: r, url: s.URL + "/releases/", version: version, dir: dir}
	if err := os.WriteFile(u.binary(), []byte(binary(version)), 0o755); err != nil {
		t.Fatal(err)
	}
	u.link = filepath.Join(t.TempDir(), "keduly")
	if err := os.Symlink(u.binary(), u.link); err != nil {
		t.Fatal(err)
	}
	return u
}

func (u *updater) binary() string { return filepath.Join(u.dir, "keduly") }

func (u *updater) run(args ...string) (string, int) {
	u.t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(append([]string{"update"}, args...), cli.Env{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, Version: u.version,
		Getenv:     func(k string) string { return map[string]string{"KEDULY_RELEASES_URL": u.url}[k] },
		Executable: func() (string, error) { return u.link, nil },
	})
	return stdout.String() + stderr.String(), code
}

// installed insists on the binary of version, alone in its directory.
func (u *updater) installed(version string) {
	u.t.Helper()
	got, err := os.ReadFile(u.binary())
	if err != nil || string(got) != binary(version) {
		u.t.Fatalf("installed %q (%v), want %s", got, err, version)
	}
	if entries, _ := os.ReadDir(u.dir); len(entries) != 1 {
		u.t.Fatalf("left behind in %s: %v", u.dir, entries)
	}
	if info, err := os.Lstat(u.link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		u.t.Fatalf("the link was replaced: %v", err)
	}
}

func TestUpdate(t *testing.T) {
	// Up to date: nothing is downloaded.
	u := newUpdater(t, "v0.3.0", "v0.3.0")
	out, code := u.run()
	if code != 0 || !strings.Contains(out, "keduly v0.3.0 已是最新版本") || len(u.r.downloads) != 0 {
		t.Fatalf("up to date: %d %q, downloads %v", code, out, u.r.downloads)
	}
	u.installed("v0.3.0")
	u = newUpdater(t, "v0.10.0", "v0.9.0")
	if out, code = u.run(); code != 0 || !strings.Contains(out, "比最新发布的 v0.9.0 还新") {
		t.Fatalf("newer than the latest: %d %q", code, out)
	}

	// --check reports and changes nothing.
	u = newUpdater(t, "v0.2.0", "v0.3.0")
	if out, code = u.run("--check"); code != 0 || !strings.Contains(out, "当前版本 v0.2.0，最新版本 v0.3.0") {
		t.Fatalf("--check: %d %q", code, out)
	}
	if out, code = u.run("--check", "--json"); code != 0 || !strings.Contains(out, `"release":"v0.3.0","newer":true,"updated":false`) {
		t.Fatalf("--check --json: %d %q", code, out)
	}
	if len(u.r.downloads) != 0 {
		t.Fatalf("--check downloaded %v", u.r.downloads)
	}
	u.installed("v0.2.0")

	// An update replaces the file the link points to, from the tag it resolved.
	out, code = u.run()
	if code != 0 {
		t.Fatalf("update: %d %s", code, out)
	}
	for _, part := range []string{"已更新 keduly v0.2.0 -> v0.3.0", u.binary(), "keduly skill install"} {
		if !strings.Contains(out, part) {
			t.Fatalf("update printed %q, without %q", out, part)
		}
	}
	u.installed("v0.3.0")
	if info, _ := os.Stat(u.binary()); info.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if !strings.Contains(strings.Join(u.r.downloads, " "), "v0.3.0/keduly-") {
		t.Fatalf("downloads %v", u.r.downloads)
	}

	// --version installs the release it names, an older one too.
	u.version = "v0.3.0"
	if out, code = u.run("--version", "0.2.0"); code != 0 || !strings.Contains(out, "v0.3.0 -> v0.2.0") {
		t.Fatalf("--version: %d %q", code, out)
	}
	u.installed("v0.2.0")
}

func TestUpdateLeavesTheBinaryOnFailure(t *testing.T) {
	u := newUpdater(t, "v0.2.0", "v0.3.0")
	u.r.badSum = true
	out, code := u.run()
	if code != 1 || !strings.Contains(out, "checksum mismatch") || !strings.Contains(out, "not changed") {
		t.Fatalf("a bad checksum: %d %q", code, out)
	}
	u.installed("v0.2.0")

	// A build from source is not a release to compare with.
	u = newUpdater(t, "dev", "v0.3.0")
	for _, version := range []string{"dev", "v0.2.1-3-gabc1234", "v0.2.1-dirty"} {
		u.version = version
		out, code = u.run()
		if code != 1 || !strings.Contains(out, "built from source") || !strings.Contains(out, "--version") {
			t.Fatalf("%s: %d %q", version, code, out)
		}
	}
	if out, code = u.run("--check"); code != 0 || !strings.Contains(out, "从源码构建") {
		t.Fatalf("--check from source: %d %q", code, out)
	}
	u.installed("dev")
	if out, code = u.run("--force"); code != 0 || !strings.Contains(out, "-> v0.3.0") {
		t.Fatalf("--force: %d %q", code, out)
	}
	u.installed("v0.3.0")

	if out, code = u.run("--check", "--force"); code != 2 {
		t.Fatalf("--check --force: %d %q", code, out)
	}
	if out, code = u.run("--version", "latest"); code != 2 || !strings.Contains(out, "must look like v1.2.3") {
		t.Fatalf("--version latest: %d %q", code, out)
	}
	u.r.latest = "nightly"
	u.version = "v0.2.0"
	if out, code = u.run(); code != 1 || !strings.Contains(out, "cannot tell the latest release") {
		t.Fatalf("a strange tag: %d %q", code, out)
	}

	// A directory it cannot write to is reported before anything is downloaded.
	if os.Geteuid() == 0 {
		return
	}
	u = newUpdater(t, "v0.2.0", "v0.3.0")
	if err := os.Chmod(u.dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(u.dir, 0o755) })
	if out, code = u.run(); code != 1 || !strings.Contains(out, "cannot write to "+u.dir) || !strings.Contains(out, "install.sh") {
		t.Fatalf("a read-only directory: %d %q", code, out)
	}
	if len(u.r.downloads) != 0 {
		t.Fatalf("downloaded %v", u.r.downloads)
	}
	u.installed("v0.2.0")
}
