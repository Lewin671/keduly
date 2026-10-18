package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// releasesURL is where the releases are; KEDULY_RELEASES_URL replaces it for a mirror.
const releasesURL = "https://github.com/Lewin671/keduly/releases"

// maxArchive bounds what a download may take; a release archive is far smaller.
const maxArchive = 256 << 20

var semverRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.]+))?$`)

type updateResult struct {
	Current string `json:"current"`
	Release string `json:"release"`
	Newer   bool   `json:"newer"`
	Updated bool   `json:"updated"`
	Path    string `json:"path,omitempty"`
	pinned  bool   // the release was named with --version
}

// update replaces the running binary with a release, as install.sh does.
func (a *app) update(args []string) error {
	fs := a.flags("update", false)
	check := fs.Bool("check", false, "only report the current and the latest version")
	version := fs.String("version", "", "install this release, e.g. v0.2.0, even an older one")
	force := fs.Bool("force", false, "install even if this version is current, or was built from source")
	if _, err := a.parseN(fs, args, 0, "[--check] [--version vX.Y.Z] [--force]"); err != nil {
		return err
	}
	if *check && (*version != "" || *force) {
		return usagef("--check cannot be combined with --version or --force")
	}
	current := a.env.Version
	released := isRelease(current)
	if !released && !*check && *version == "" && !*force {
		return fmt.Errorf("this keduly (version %s) was built from source; pass --version vX.Y.Z to install a release over it, or --force for the latest", current)
	}
	base := strings.TrimSuffix(a.env.Getenv("KEDULY_RELEASES_URL"), "/")
	if base == "" {
		base = releasesURL
	}

	res := updateResult{Current: current, pinned: *version != ""}
	if res.pinned {
		res.Release = "v" + strings.TrimPrefix(*version, "v")
		if !semverRE.MatchString(res.Release) {
			return usagef("--version must look like v1.2.3, not %q", *version)
		}
	} else {
		tag, err := a.latestTag(base)
		if err != nil {
			return err
		}
		res.Release = tag
	}
	res.Newer = !released || compareVersions(res.Release, current) > 0

	if *check {
		return a.updateReport(res)
	}
	// Without --version only a newer release replaces this one; --version names the one wanted.
	if !*force && (res.Release == current || (!res.pinned && !res.Newer)) {
		return a.updateReport(res)
	}
	target, err := a.executable()
	if err != nil {
		return err
	}
	if err := a.install(base, res.Release, target); err != nil {
		return err
	}
	res.Updated, res.Path = true, target
	return a.updateReport(res)
}

func (a *app) updateReport(r updateResult) error {
	switch {
	case a.json:
		data, _ := json.Marshal(r)
		a.printf("%s\n", data)
	case r.Updated:
		a.printf("已更新 keduly %s -> %s（%s）\n", r.Current, r.Release, r.Path)
		a.printf("如果用 `keduly skill install` 安装过 agent skill，请带上当时的参数再运行一次，让 skill 与新版本一致\n")
	case !isRelease(r.Current):
		a.printf("keduly %s 是从源码构建的，最新版本 %s；`keduly update --force` 换成发布版\n", r.Current, r.Release)
	case r.Newer:
		a.printf("当前版本 %s，最新版本 %s；运行 `keduly update` 更新\n", r.Current, r.Release)
	case r.Release == r.Current && !r.pinned:
		a.printf("keduly %s 已是最新版本\n", r.Current)
	case r.Release == r.Current:
		a.printf("keduly %s 已是这个版本，无需更新\n", r.Current)
	default:
		a.printf("keduly %s 比最新发布的 %s 还新，无需更新\n", r.Current, r.Release)
	}
	return nil
}

// latestTag asks which release is the latest without downloading it: the
// latest release page redirects to the page of its tag.
func (a *app) latestTag(base string) (string, error) {
	client := *a.env.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := a.fetch(&client, base+"/latest")
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("%s did not redirect to a release (%s)", base+"/latest", resp.Status)
	}
	_, tag, ok := strings.Cut(loc.Path, "/releases/tag/")
	if !ok || !semverRE.MatchString(tag) {
		return "", fmt.Errorf("cannot tell the latest release from %s", loc)
	}
	return tag, nil
}

// executable is the file of the running binary, links resolved, so that the
// replacement lands where the link points and not over the link.
func (a *app) executable() (string, error) {
	find := a.env.Executable
	if find == nil {
		find = os.Executable
	}
	exe, err := find()
	if err != nil {
		return "", fmt.Errorf("cannot find the running keduly: %v", err)
	}
	return filepath.EvalSymlinks(exe)
}

// install downloads the release archive for this platform, checks it against
// checksums.txt and moves its binary over target. Until the final rename the
// old binary is untouched; on any failure only the temporary file is removed.
func (a *app) install(base, tag, target string) error {
	if (runtime.GOOS != "darwin" && runtime.GOOS != "linux") || (runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64") {
		return fmt.Errorf("there are no releases for %s/%s; build from source instead", runtime.GOOS, runtime.GOARCH)
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".keduly-update-*")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("cannot write to %s; run `keduly update` again as a user who can, or reinstall with install.sh", dir)
		}
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	archive := fmt.Sprintf("keduly-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	from := base + "/download/" + tag + "/"
	client := *a.env.HTTP
	client.Timeout = 10 * time.Minute
	sums, err := a.download(&client, from+"checksums.txt")
	if err != nil {
		return err
	}
	want, err := checksumOf(sums, archive)
	if err != nil {
		return err
	}
	data, err := a.download(&client, from+archive)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s; keduly was not changed", archive, want, got)
	}
	if err := extractBinary(data, tmp); err != nil {
		return fmt.Errorf("%s: %v", archive, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	// A rename is atomic, and a keduly still running keeps the file it opened.
	return os.Rename(tmp.Name(), target)
}

func (a *app) fetch(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "keduly/"+a.env.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %v", url, err)
	}
	return resp, nil
}

func (a *app) download(client *http.Client, url string) ([]byte, error) {
	resp, err := a.fetch(client, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot download %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive+1))
	if err != nil {
		return nil, fmt.Errorf("cannot download %s: %v", url, err)
	}
	if len(data) > maxArchive {
		return nil, fmt.Errorf("%s is larger than any release", url)
	}
	return data, nil
}

// checksumOf finds name in checksums.txt, whose lines are "SHA256  NAME" or "SHA256 *NAME".
func checksumOf(sums []byte, name string) (string, error) {
	lines := bufio.NewScanner(bytes.NewReader(sums))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && (fields[1] == name || fields[1] == "*"+name) {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt does not list %s", name)
}

// extractBinary copies the keduly file of a release archive to w.
func extractBinary(archive []byte, w io.Writer) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return errors.New("the archive has no keduly binary")
		}
		if err != nil {
			return err
		}
		if path.Clean(h.Name) == "keduly" && h.Typeflag == tar.TypeReg {
			_, err := io.Copy(w, tr)
			return err
		}
	}
}

// isRelease tells a released version from a build of a source tree: "dev",
// or what git describe adds between tags ("v0.2.1-3-gabc1234", "-dirty").
func isRelease(v string) bool {
	m := semverRE.FindStringSubmatch(v)
	return m != nil && !strings.HasSuffix(v, "-dirty")
}

// compareVersions orders two versions semver's way, prereleases before the
// release they lead to.
func compareVersions(a, b string) int {
	ma, mb := semverRE.FindStringSubmatch(a), semverRE.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return cmp.Compare(x, y)
		}
	}
	switch pa, pb := ma[4], mb[4]; {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	default:
		return comparePrerelease(strings.Split(pa, "."), strings.Split(pb, "."))
	}
}

func comparePrerelease(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		x, errX := strconv.Atoi(a[i])
		y, errY := strconv.Atoi(b[i])
		switch {
		case errX == nil && errY == nil:
			if x != y {
				return cmp.Compare(x, y)
			}
		case errX == nil:
			return -1
		case errY == nil:
			return 1
		case a[i] != b[i]:
			return strings.Compare(a[i], b[i])
		}
	}
	return cmp.Compare(len(a), len(b))
}
