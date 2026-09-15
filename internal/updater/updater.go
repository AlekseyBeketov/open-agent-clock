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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRepository           = "AlekseyBeketov/open-agent-clock"
	maxMetadataBytes            = 1 << 20
	maxArchiveBytes             = 128 << 20
	maxUncompressedArchiveBytes = 128 << 20
	maxBinaryBytes              = 64 << 20
)

var (
	ErrUnsupportedPlatform = errors.New("updates are supported only on macOS arm64 and amd64")
	ErrUnsupportedVersion  = errors.New("self-update requires a released semantic version; dev or unknown versions cannot be installed")
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type TempFile interface {
	io.Writer
	Name() string
	Sync() error
	Chmod(os.FileMode) error
	Close() error
}

type FileSystem interface {
	Lstat(string) (os.FileInfo, error)
	ReadFile(string) ([]byte, error)
	CreateTemp(string, string) (TempFile, error)
	Rename(string, string) error
	Remove(string) error
	SyncDir(string) error
}

type OSFileSystem struct{}

func (OSFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }
func (OSFileSystem) ReadFile(name string) ([]byte, error)   { return os.ReadFile(name) }
func (OSFileSystem) CreateTemp(dir, pattern string) (TempFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (OSFileSystem) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (OSFileSystem) Remove(name string) error             { return os.Remove(name) }
func (OSFileSystem) SyncDir(name string) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

type Service struct {
	HTTP           HTTPClient
	FS             FileSystem
	ExecutablePath func() (string, error)
	GOOS           string
	GOARCH         string
	APIBaseURL     string
	Repository     string
	Now            func() time.Time
}

type Result struct {
	Operation      string    `json:"operation"`
	Status         string    `json:"status"`
	CurrentVersion string    `json:"current_version,omitempty"`
	LatestVersion  string    `json:"latest_version,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	preRelease []string
}

func New(repository string) *Service {
	if strings.TrimSpace(repository) == "" {
		repository = defaultRepository
	}
	return &Service{
		HTTP:           &http.Client{Timeout: 10 * time.Second},
		FS:             OSFileSystem{},
		ExecutablePath: os.Executable,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		APIBaseURL:     "https://api.github.com/repos/" + repository + "/releases/latest",
		Repository:     repository,
		Now:            time.Now,
	}
}

func (service *Service) Check(ctx context.Context, current string) (Result, error) {
	result := service.newResult("check", current)
	if err := service.validatePlatform(); err != nil {
		result.Status = "unsupported"
		result.Reason = err.Error()
		return result, err
	}
	currentVersion, err := parseSemanticVersion(current)
	if err != nil {
		result.Status = "unsupported"
		result.Reason = ErrUnsupportedVersion.Error()
		return result, fmt.Errorf("%w: %v", ErrUnsupportedVersion, err)
	}
	releaseInfo, err := service.fetchRelease(ctx)
	if err != nil {
		result.Status = "failed"
		result.Reason = err.Error()
		return result, err
	}
	latestVersion, err := parseSemanticVersion(releaseInfo.TagName)
	if err != nil {
		result.Status = "failed"
		result.Reason = "latest release has an invalid semantic version"
		return result, fmt.Errorf("invalid latest release tag %q: %w", releaseInfo.TagName, err)
	}
	result.LatestVersion = releaseInfo.TagName
	if compareVersions(latestVersion, currentVersion) <= 0 {
		result.Status = "current"
		result.Reason = "already up to date"
		return result, nil
	}
	result.Status = "available"
	result.Reason = "new release available"
	return result, nil
}

func (service *Service) Install(ctx context.Context, current string, confirm bool) (Result, error) {
	result := service.newResult("install", current)
	if !confirm {
		result.Status = "cancelled"
		result.Reason = "installation requires explicit --confirm; nothing was changed"
		return result, errors.New(result.Reason)
	}
	if err := service.validatePlatform(); err != nil {
		result.Status = "unsupported"
		result.Reason = err.Error()
		return result, err
	}
	if _, err := parseSemanticVersion(current); err != nil {
		result.Status = "unsupported"
		result.Reason = ErrUnsupportedVersion.Error()
		return result, fmt.Errorf("%w: %v", ErrUnsupportedVersion, err)
	}
	if service.FS == nil || service.ExecutablePath == nil {
		return service.failedResult(result, "updater filesystem or executable seam is not configured", errors.New("updater filesystem or executable seam is not configured"))
	}
	executable, err := service.ExecutablePath()
	if err != nil {
		return service.failedResult(result, "resolve current executable: "+err.Error(), err)
	}
	if err := service.prepareInstall(executable); err != nil {
		return service.failedResult(result, err.Error(), err)
	}

	releaseInfo, err := service.fetchRelease(ctx)
	if err != nil {
		return service.failedResult(result, err.Error(), err)
	}
	currentVersion, _ := parseSemanticVersion(current)
	latestVersion, err := parseSemanticVersion(releaseInfo.TagName)
	if err != nil {
		return service.failedResult(result, "latest release has an invalid semantic version", fmt.Errorf("invalid latest release tag %q: %w", releaseInfo.TagName, err))
	}
	result.LatestVersion = releaseInfo.TagName
	if compareVersions(latestVersion, currentVersion) <= 0 {
		result.Status = "current"
		result.Reason = "already up to date; executable was not changed"
		return result, nil
	}

	archiveName := fmt.Sprintf("open-agent-clock_darwin_%s.tar.gz", service.GOARCH)
	archiveAsset, ok := findAsset(releaseInfo.Assets, archiveName)
	if !ok {
		return service.failedResult(result, "the exact macOS architecture release archive was not published", fmt.Errorf("release asset %q was not found", archiveName))
	}
	checksumAsset, ok := findAsset(releaseInfo.Assets, "checksums.txt")
	if !ok {
		return service.failedResult(result, "checksums.txt is mandatory; refusing to install", errors.New("release asset checksums.txt was not found"))
	}
	archive, err := service.fetchAsset(ctx, archiveAsset, maxArchiveBytes)
	if err != nil {
		return service.failedResult(result, "download archive: "+err.Error(), err)
	}
	checksums, err := service.fetchAsset(ctx, checksumAsset, maxMetadataBytes)
	if err != nil {
		return service.failedResult(result, "download checksums.txt: "+err.Error(), err)
	}
	if err := verifyChecksum(archive, checksums, archiveName); err != nil {
		return service.failedResult(result, err.Error(), err)
	}
	binary, err := extractExecutable(archive)
	if err != nil {
		return service.failedResult(result, err.Error(), err)
	}
	if err := service.replaceExecutable(executable, binary); err != nil {
		return service.failedResult(result, err.Error(), err)
	}
	result.Status = "installed"
	result.Reason = "verified release installed atomically"
	return result, nil
}

func (service *Service) newResult(operation, current string) Result {
	now := time.Now()
	if service.Now != nil {
		now = service.Now()
	}
	return Result{Operation: operation, CurrentVersion: current, RecordedAt: now.UTC()}
}

func (service *Service) failedResult(result Result, reason string, err error) (Result, error) {
	result.Status = "failed"
	result.Reason = reason
	return result, err
}

func (service *Service) validatePlatform() error {
	if service.GOOS != "darwin" || (service.GOARCH != "arm64" && service.GOARCH != "amd64") {
		return ErrUnsupportedPlatform
	}
	return nil
}

func (service *Service) fetchRelease(ctx context.Context) (release, error) {
	if service.HTTP == nil {
		return release{}, errors.New("updater HTTP seam is not configured")
	}
	endpoint := service.APIBaseURL
	if strings.TrimSpace(endpoint) == "" {
		return release{}, errors.New("updater GitHub release endpoint is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return release{}, fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "open-agent-clock-updater")
	response, err := service.HTTP.Do(request)
	if err != nil {
		return release{}, fmt.Errorf("request latest release: %w", err)
	}
	if response == nil || response.Body == nil {
		return release{}, errors.New("GitHub returned an empty response")
	}
	if response.ContentLength > maxMetadataBytes {
		return release{}, fmt.Errorf("release metadata exceeds the %d-byte limit", maxMetadataBytes)
	}
	defer response.Body.Close()
	contents, err := readBounded(response.Body, maxMetadataBytes)
	if err != nil {
		return release{}, fmt.Errorf("read release metadata: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return release{}, fmt.Errorf("GitHub release request returned HTTP %d", response.StatusCode)
	}
	var value release
	if err := json.Unmarshal(contents, &value); err != nil {
		return release{}, fmt.Errorf("decode release metadata: %w", err)
	}
	if strings.TrimSpace(value.TagName) == "" {
		return release{}, errors.New("GitHub release metadata did not contain a tag")
	}
	return value, nil
}

func (service *Service) fetchAsset(ctx context.Context, value asset, limit int64) ([]byte, error) {
	if err := service.validateAssetURL(value.URL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, value.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create asset request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "open-agent-clock-updater")
	response, err := service.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("GitHub returned an empty asset response")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("asset %q returned HTTP %d", value.Name, response.StatusCode)
	}
	if response.ContentLength > limit {
		return nil, fmt.Errorf("asset %q exceeds the %d-byte limit", value.Name, limit)
	}
	contents, err := readBounded(response.Body, limit)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func (service *Service) validateAssetURL(raw string) error {
	value, err := url.Parse(raw)
	if err != nil || value.Scheme == "" || value.Hostname() == "" || value.User != nil {
		return errors.New("release asset URL is invalid")
	}
	if value.Scheme != "http" && value.Scheme != "https" {
		return errors.New("release asset URL must use HTTP or HTTPS")
	}
	base, err := url.Parse(service.APIBaseURL)
	if err != nil || base.Hostname() == "" {
		return errors.New("updater GitHub release endpoint is invalid")
	}
	if base.Scheme == "https" && value.Scheme != "https" {
		return errors.New("HTTPS release metadata cannot select an HTTP asset")
	}
	baseHost := strings.ToLower(base.Hostname())
	assetHost := strings.ToLower(value.Hostname())
	if baseHost == "api.github.com" {
		if assetHost != "api.github.com" && assetHost != "github.com" {
			return errors.New("release asset host is not GitHub")
		}
	} else if assetHost != baseHost {
		return errors.New("release asset host does not match the release endpoint")
	}
	return nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("response exceeds the %d-byte limit", limit)
	}
	return contents, nil
}

func findAsset(assets []asset, name string) (asset, bool) {
	var match asset
	found := false
	for _, value := range assets {
		if value.Name != name {
			continue
		}
		if found || strings.TrimSpace(value.URL) == "" {
			return asset{}, false
		}
		match = value
		found = true
	}
	return match, found
}

func verifyChecksum(archive, checksums []byte, archiveName string) error {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name != archiveName {
			continue
		}
		expected := strings.ToLower(fields[0])
		if len(expected) != sha256.Size*2 {
			return errors.New("checksum entry has an invalid SHA-256 digest")
		}
		if _, err := hex.DecodeString(expected); err != nil {
			return errors.New("checksum entry has an invalid SHA-256 digest")
		}
		actualBytes := sha256.Sum256(archive)
		actual := hex.EncodeToString(actualBytes[:])
		if actual != expected {
			return fmt.Errorf("checksum verification failed for %s", archiveName)
		}
		return nil
	}
	return fmt.Errorf("checksums.txt has no entry for %s", archiveName)
}

func extractExecutable(archive []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer reader.Close()
	limited := &io.LimitedReader{R: reader, N: maxUncompressedArchiveBytes + 1}
	tarReader := tar.NewReader(limited)
	var executable []byte
	found := false
	entries := 0
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		entries++
		if entries > 256 {
			return nil, errors.New("release archive contains too many entries")
		}
		name, err := safeArchiveName(header.Name)
		if err != nil {
			return nil, err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA:
			if name != "open-agent-clock" {
				if _, err := io.Copy(io.Discard, tarReader); err != nil {
					return nil, fmt.Errorf("read archive entry %q: %w", name, err)
				}
				continue
			}
			if found {
				return nil, errors.New("release archive contains duplicate open-agent-clock files")
			}
			if header.Size < 1 || header.Size > maxBinaryBytes {
				return nil, errors.New("release archive executable has an invalid size")
			}
			executable, err = io.ReadAll(io.LimitReader(tarReader, maxBinaryBytes+1))
			if err != nil {
				return nil, fmt.Errorf("read executable from archive: %w", err)
			}
			if int64(len(executable)) != header.Size {
				return nil, errors.New("release archive executable was truncated")
			}
			found = true
		default:
			return nil, fmt.Errorf("release archive entry %q has an unsafe link or special type", name)
		}
	}
	if !found {
		return nil, errors.New("release archive does not contain open-agent-clock")
	}
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return nil, fmt.Errorf("read release archive: %w", err)
	}
	if limited.N == 0 {
		return nil, fmt.Errorf("release archive exceeds the %d-byte uncompressed limit", maxUncompressedArchiveBytes)
	}
	return executable, nil
}

func safeArchiveName(value string) (string, error) {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("release archive contains an unsafe path %q", value)
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == ".." {
			return "", fmt.Errorf("release archive contains an unsafe path %q", value)
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("release archive contains an unsafe path %q", value)
	}
	return cleaned, nil
}

func (service *Service) prepareInstall(executable string) error {
	info, err := service.FS.Lstat(executable)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	if info == nil {
		return errors.New("inspect current executable: filesystem returned no file information")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("refusing to replace a symlink; update the package manager installation instead")
	}
	if !info.Mode().IsRegular() {
		return errors.New("refusing to replace a non-regular executable")
	}
	if isPackageManagedPath(executable) {
		return errors.New("refusing to replace a package-manager-managed executable; update it with its package manager")
	}
	directory := filepath.Dir(executable)
	temporary, err := service.FS.CreateTemp(directory, ".open-agent-clock-update-writable-*")
	if err != nil {
		return fmt.Errorf("executable directory is not writable: %w", err)
	}
	name := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = service.FS.Remove(name)
		return fmt.Errorf("close writable-directory probe: %w", err)
	}
	if err := service.FS.Remove(name); err != nil {
		return fmt.Errorf("remove writable-directory probe: %w", err)
	}
	return nil
}

func isPackageManagedPath(value string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(value))
	for _, marker := range []string{"/Cellar/", "/Caskroom/", "/MacPorts/"} {
		if strings.Contains(cleaned, marker) {
			return true
		}
	}
	return false
}

func (service *Service) replaceExecutable(executable string, contents []byte) error {
	directory := filepath.Dir(executable)
	stage, err := service.FS.CreateTemp(directory, ".open-agent-clock-update-*")
	if err != nil {
		return fmt.Errorf("create staged executable: %w", err)
	}
	stageName := stage.Name()
	defer func() { _ = service.FS.Remove(stageName) }()
	if _, err := stage.Write(contents); err != nil {
		_ = stage.Close()
		return fmt.Errorf("write staged executable: %w", err)
	}
	if err := stage.Sync(); err != nil {
		_ = stage.Close()
		return fmt.Errorf("sync staged executable: %w", err)
	}
	if err := stage.Chmod(0o755); err != nil {
		_ = stage.Close()
		return fmt.Errorf("set staged executable permissions: %w", err)
	}
	if err := stage.Close(); err != nil {
		return fmt.Errorf("close staged executable: %w", err)
	}

	backup, err := service.reserveTemporaryPath(directory, ".open-agent-clock-backup-*")
	if err != nil {
		return fmt.Errorf("prepare executable rollback: %w", err)
	}
	backupName := backup
	defer func() { _ = service.FS.Remove(backupName) }()
	if err := service.FS.Rename(executable, backupName); err != nil {
		return fmt.Errorf("stage current executable: %w", err)
	}
	if err := service.FS.Rename(stageName, executable); err != nil {
		if restoreErr := service.FS.Rename(backupName, executable); restoreErr != nil {
			return fmt.Errorf("install staged executable: %w; restore current executable: %v", err, restoreErr)
		}
		return fmt.Errorf("install staged executable: %w", err)
	}
	if err := service.FS.SyncDir(directory); err != nil {
		rollbackErr := service.rollbackReplacement(executable, stageName, backupName, directory)
		if rollbackErr != nil {
			return fmt.Errorf("sync executable directory: %w; rollback failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("sync executable directory: %w", err)
	}
	if err := service.FS.Remove(backupName); err != nil {
		// The new executable is already committed. Keep the backup as a recovery
		// aid and report the non-secret path through the error-free result reason.
		return nil
	}
	backupName = ""
	return nil
}

func (service *Service) rollbackReplacement(executable, stage, backup, directory string) error {
	if err := service.FS.Rename(executable, stage); err != nil {
		return err
	}
	if err := service.FS.Rename(backup, executable); err != nil {
		return err
	}
	return service.FS.SyncDir(directory)
}

func (service *Service) reserveTemporaryPath(directory, pattern string) (string, error) {
	file, err := service.FS.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = service.FS.Remove(name)
		return "", err
	}
	if err := service.FS.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "v") || strings.HasPrefix(value, "V") {
		value = value[1:]
	}
	if value == "" || strings.EqualFold(value, "dev") || strings.EqualFold(value, "unknown") {
		return semanticVersion{}, errors.New("version is not a released semantic version")
	}
	withoutBuild := strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(withoutBuild, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return semanticVersion{}, errors.New("semantic version must have major, minor, and patch components")
	}
	values := [3]int{}
	for index, part := range core {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semanticVersion{}, errors.New("semantic version contains an invalid numeric component")
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return semanticVersion{}, errors.New("semantic version contains an invalid numeric component")
		}
		values[index] = number
	}
	result := semanticVersion{major: values[0], minor: values[1], patch: values[2]}
	if len(parts) == 2 {
		if parts[1] == "" {
			return semanticVersion{}, errors.New("semantic version has an empty pre-release")
		}
		result.preRelease = strings.Split(parts[1], ".")
		for _, identifier := range result.preRelease {
			if identifier == "" {
				return semanticVersion{}, errors.New("semantic version has an empty pre-release identifier")
			}
		}
	}
	return result, nil
}

func compareVersions(left, right semanticVersion) int {
	for _, pair := range [][2]int{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(left.preRelease) == 0 && len(right.preRelease) == 0 {
		return 0
	}
	if len(left.preRelease) == 0 {
		return 1
	}
	if len(right.preRelease) == 0 {
		return -1
	}
	for index := 0; index < len(left.preRelease) && index < len(right.preRelease); index++ {
		leftID, rightID := left.preRelease[index], right.preRelease[index]
		leftNumber, leftErr := strconv.Atoi(leftID)
		rightNumber, rightErr := strconv.Atoi(rightID)
		if leftErr == nil && rightErr == nil {
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
			continue
		}
		if leftErr == nil {
			return -1
		}
		if rightErr == nil {
			return 1
		}
		if leftID < rightID {
			return -1
		}
		if leftID > rightID {
			return 1
		}
	}
	if len(left.preRelease) < len(right.preRelease) {
		return -1
	}
	if len(left.preRelease) > len(right.preRelease) {
		return 1
	}
	return 0
}
