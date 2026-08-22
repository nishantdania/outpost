package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultReleaseURL = "https://api.github.com/repos/nishantdania/outpost/releases/latest"

type Result struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
}

type Options struct {
	Component      string
	CurrentVersion string
	Executable     string
	ReleaseURL     string
	Client         *http.Client
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func Apply(ctx context.Context, options Options) (Result, error) {
	release, err := latest(ctx, options)
	if err != nil {
		return Result{}, err
	}

	result := Result{CurrentVersion: options.CurrentVersion, LatestVersion: release.TagName}
	if options.CurrentVersion == release.TagName {
		return result, nil
	}

	archiveName := fmt.Sprintf("%s_%s_%s.tar.gz", options.Component, runtime.GOOS, runtime.GOARCH)
	archiveURL, checksumsURL := assets(release, archiveName)
	if archiveURL == "" || checksumsURL == "" {
		return Result{}, fmt.Errorf("release %s does not contain %s", release.TagName, archiveName)
	}

	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	checksums, err := download(ctx, client, checksumsURL)
	if err != nil {
		return Result{}, err
	}
	archive, err := download(ctx, client, archiveURL)
	if err != nil {
		return Result{}, err
	}
	if err := verify(archive, archiveName, checksums); err != nil {
		return Result{}, err
	}
	binary, err := extract(archive, options.Component)
	if err != nil {
		return Result{}, err
	}
	if err := replace(options.Executable, binary); err != nil {
		return Result{}, err
	}
	result.Updated = true
	return result, nil
}

func latest(ctx context.Context, options Options) (release, error) {
	url := options.ReleaseURL
	if url == "" {
		url = defaultReleaseURL
	}
	client := options.Client
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return release{}, fmt.Errorf("get latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("get latest release: %s", response.Status)
	}
	var value release
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if value.TagName == "" {
		return release{}, fmt.Errorf("latest release has no tag")
	}
	return value, nil
}

func assets(value release, archiveName string) (string, string) {
	var archiveURL, checksumsURL string
	for _, asset := range value.Assets {
		switch asset.Name {
		case archiveName:
			archiveURL = asset.DownloadURL
		case "checksums.txt":
			checksumsURL = asset.DownloadURL
		}
	}
	return archiveURL, checksumsURL
}

func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func verify(archive []byte, name string, checksums []byte) error {
	var expected string
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum not found for %s", name)
	}
	digest := sha256.Sum256(archive)
	if !strings.EqualFold(expected, hex.EncodeToString(digest[:])) {
		return fmt.Errorf("checksum verification failed for %s", name)
	}
	return nil
}

func extract(archive []byte, component string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if header.Name == component && header.Typeflag == tar.TypeReg {
			return io.ReadAll(tarReader)
		}
	}
	return nil, fmt.Errorf("archive does not contain %s", component)
}

func replace(executable string, binary []byte) error {
	directory := filepath.Dir(executable)
	file, err := os.CreateTemp(directory, ".outpost-")
	if err != nil {
		return fmt.Errorf("create temporary binary: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(binary); err != nil {
		file.Close()
		return fmt.Errorf("write temporary binary: %w", err)
	}
	if err := file.Chmod(0o755); err != nil {
		file.Close()
		return fmt.Errorf("set temporary binary permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary binary: %w", err)
	}
	backup := executable + ".old"
	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("backup executable: %w", err)
	}
	if err := os.Rename(temporary, executable); err != nil {
		_ = os.Rename(backup, executable)
		return fmt.Errorf("replace executable: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}
