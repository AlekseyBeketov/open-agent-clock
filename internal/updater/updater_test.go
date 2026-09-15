package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeHTTPResponse struct {
	status int
	body   []byte
}

type fakeHTTPClient struct {
	responses map[string]fakeHTTPResponse
	calls     []string
}

func (client *fakeHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.calls = append(client.calls, request.URL.String())
	value, ok := client.responses[request.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected fake request %s", request.URL)
	}
	status := value.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode:    status,
		Body:          io.NopCloser(bytes.NewReader(value.body)),
		ContentLength: int64(len(value.body)),
		Header:        make(http.Header),
		Request:       request,
	}, nil
}

type fakeFileSystem struct {
	OSFileSystem
	renameCalls  int
	failRenameAt int
	failSync     bool
}

func (fs *fakeFileSystem) Rename(oldPath, newPath string) error {
	fs.renameCalls++
	if fs.failRenameAt > 0 && fs.renameCalls == fs.failRenameAt {
		return errors.New("synthetic rename failure")
	}
	return fs.OSFileSystem.Rename(oldPath, newPath)
}

func (fs *fakeFileSystem) SyncDir(name string) error {
	if fs.failSync {
		return errors.New("synthetic directory sync failure")
	}
	return fs.OSFileSystem.SyncDir(name)
}

func TestCheckReportsNewReleaseWithoutTouchingFilesystem(t *testing.T) {
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, OSFileSystem{}, nil)
	client.responses[service.APIBaseURL] = fakeHTTPResponse{body: []byte(`{"tag_name":"v1.2.0","assets":[]}`)}

	result, err := service.Check(context.Background(), "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "available" || result.CurrentVersion != "v1.1.0" || result.LatestVersion != "v1.2.0" {
		t.Fatalf("check result = %+v", result)
	}
	if len(client.calls) != 1 || client.calls[0] != service.APIBaseURL {
		t.Fatalf("fake requests = %#v", client.calls)
	}
}

func TestCheckRejectsDevelopmentVersionBeforeNetwork(t *testing.T) {
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, OSFileSystem{}, nil)

	result, err := service.Check(context.Background(), "dev")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("error = %v", err)
	}
	if result.Status != "unsupported" || len(client.calls) != 0 {
		t.Fatalf("result = %+v, calls = %#v", result, client.calls)
	}
}

func TestInstallRequiresConfirmationBeforeFilesystemOrNetwork(t *testing.T) {
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	fs := &fakeFileSystem{}
	service := testService(client, fs, nil)

	result, err := service.Install(context.Background(), "v1.1.0", false)
	if err == nil || result.Status != "cancelled" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if len(client.calls) != 0 || fs.renameCalls != 0 {
		t.Fatalf("confirmation touched seams: requests=%#v renames=%d", client.calls, fs.renameCalls)
	}
}

func TestInstallVerifiesChecksumAndAtomicallyReplacesExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "open-agent-clock")
	oldContents := []byte("old executable")
	newContents := []byte("new verified executable")
	if err := os.WriteFile(executable, oldContents, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := tarGzip(t, tarEntry{name: "open-agent-clock", data: newContents})
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, &fakeFileSystem{}, func() (string, error) { return executable, nil })
	configureRelease(t, service, client, archive)

	result, err := service.Install(context.Background(), "v1.1.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "installed" || result.LatestVersion != "v1.2.0" {
		t.Fatalf("install result = %+v", result)
	}
	contents, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, newContents) {
		t.Fatalf("installed bytes = %q, want %q", contents, newContents)
	}
	info, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("installed permissions = %o", info.Mode().Perm())
	}
	if len(client.calls) != 3 {
		t.Fatalf("fake requests = %#v", client.calls)
	}
}

func TestInstallChecksumFailurePreservesExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "open-agent-clock")
	oldContents := []byte("old executable")
	if err := os.WriteFile(executable, oldContents, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := tarGzip(t, tarEntry{name: "open-agent-clock", data: []byte("new executable")})
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, &fakeFileSystem{}, func() (string, error) { return executable, nil })
	configureRelease(t, service, client, append([]byte(nil), archive...))
	checksumURL := service.APIBaseURL + "/checksums"
	client.responses[checksumURL] = fakeHTTPResponse{body: []byte(strings.Repeat("0", 64) + "  open-agent-clock_darwin_arm64.tar.gz\n")}

	result, err := service.Install(context.Background(), "v1.1.0", true)
	if err == nil || result.Status != "failed" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	contents, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(contents, oldContents) {
		t.Fatalf("executable changed after checksum failure: %q", contents)
	}
}

func TestInstallRejectsUnsafeArchiveAndPreservesExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "open-agent-clock")
	oldContents := []byte("old executable")
	if err := os.WriteFile(executable, oldContents, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := tarGzip(t, tarEntry{name: "../open-agent-clock", data: []byte("unsafe")})
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, &fakeFileSystem{}, func() (string, error) { return executable, nil })
	configureRelease(t, service, client, archive)

	result, err := service.Install(context.Background(), "v1.1.0", true)
	if err == nil || result.Status != "failed" || !strings.Contains(result.Reason, "unsafe path") {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	contents, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(contents, oldContents) {
		t.Fatalf("executable changed after unsafe archive: %q", contents)
	}
}

func TestInstallRollsBackWhenAtomicReplacementRenameFails(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "open-agent-clock")
	oldContents := []byte("old executable")
	if err := os.WriteFile(executable, oldContents, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := tarGzip(t, tarEntry{name: "open-agent-clock", data: []byte("new executable")})
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	fs := &fakeFileSystem{failRenameAt: 2}
	service := testService(client, fs, func() (string, error) { return executable, nil })
	configureRelease(t, service, client, archive)

	result, err := service.Install(context.Background(), "v1.1.0", true)
	if err == nil || result.Status != "failed" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	contents, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(contents, oldContents) {
		t.Fatalf("rollback did not restore executable: %q", contents)
	}
}

func TestInstallRejectsAssetOutsideReleaseHost(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "open-agent-clock")
	if err := os.WriteFile(executable, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := &fakeHTTPClient{responses: map[string]fakeHTTPResponse{}}
	service := testService(client, &fakeFileSystem{}, func() (string, error) { return executable, nil })
	metadata, err := json.Marshal(release{TagName: "v1.2.0", Assets: []asset{{Name: "open-agent-clock_darwin_arm64.tar.gz", URL: "https://attacker.example/archive"}, {Name: "checksums.txt", URL: service.APIBaseURL + "/checksums"}}})
	if err != nil {
		t.Fatal(err)
	}
	client.responses[service.APIBaseURL] = fakeHTTPResponse{body: metadata}
	client.responses[service.APIBaseURL+"/checksums"] = fakeHTTPResponse{body: []byte("unused")}

	result, err := service.Install(context.Background(), "v1.1.0", true)
	if err == nil || result.Status != "failed" || !strings.Contains(result.Reason, "host") {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if len(client.calls) != 1 {
		t.Fatalf("unexpected requests after hostile metadata: %#v", client.calls)
	}
}

func TestExtractExecutableRejectsLinksAndDuplicateTarget(t *testing.T) {
	linkArchive := tarGzip(t, tarEntry{name: "open-agent-clock", typeflag: tar.TypeSymlink, linkname: "elsewhere"})
	if _, err := extractExecutable(linkArchive); err == nil || !strings.Contains(err.Error(), "unsafe link") {
		t.Fatalf("symlink archive error = %v", err)
	}
	duplicateArchive := tarGzip(t, tarEntry{name: "open-agent-clock", data: []byte("one")}, tarEntry{name: "open-agent-clock", data: []byte("two")})
	if _, err := extractExecutable(duplicateArchive); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate archive error = %v", err)
	}
}

func TestSemanticVersionComparisonFollowsReleaseOrdering(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{"v1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, value := range cases {
		left, leftErr := parseSemanticVersion(value.left)
		right, rightErr := parseSemanticVersion(value.right)
		if leftErr != nil || rightErr != nil {
			t.Fatalf("parse %q/%q: %v/%v", value.left, value.right, leftErr, rightErr)
		}
		if got := compareVersions(left, right); got != value.want {
			t.Errorf("compare(%q, %q) = %d, want %d", value.left, value.right, got, value.want)
		}
	}
}

func testService(client HTTPClient, fs FileSystem, executable func() (string, error)) *Service {
	return &Service{
		HTTP:           client,
		FS:             fs,
		ExecutablePath: executable,
		GOOS:           "darwin",
		GOARCH:         "arm64",
		APIBaseURL:     "https://updates.example/releases/latest",
		Now:            func() time.Time { return time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC) },
	}
}

func configureRelease(t *testing.T, service *Service, client *fakeHTTPClient, archive []byte) {
	t.Helper()
	archiveName := "open-agent-clock_darwin_arm64.tar.gz"
	archiveURL := service.APIBaseURL + "/archive"
	checksumURL := service.APIBaseURL + "/checksums"
	checksum := sha256.Sum256(archive)
	metadata, err := json.Marshal(release{TagName: "v1.2.0", Assets: []asset{{Name: archiveName, URL: archiveURL}, {Name: "checksums.txt", URL: checksumURL}}})
	if err != nil {
		t.Fatal(err)
	}
	client.responses[service.APIBaseURL] = fakeHTTPResponse{body: metadata}
	client.responses[archiveURL] = fakeHTTPResponse{body: archive}
	client.responses[checksumURL] = fakeHTTPResponse{body: []byte(hex.EncodeToString(checksum[:]) + "  " + archiveName + "\n")}
}

type tarEntry struct {
	name     string
	data     []byte
	typeflag byte
	linkname string
}

func tarGzip(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.data)), Typeflag: entry.typeflag, Linkname: entry.linkname}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(entry.data) > 0 {
			if _, err := tarWriter.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
