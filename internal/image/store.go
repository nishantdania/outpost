package image

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/outpost"
)

const (
	maxArchiveBytes = int64(256 << 20)
	maxContextBytes = int64(64 << 20)
	maxEntries      = 10000
	maxPathBytes    = 1024
	maxFileBytes    = int64(1 << 30)
	minRootFSBytes  = int64(1 << 30)
	maxRootFSBytes  = int64(8 << 30)
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
	RunIO(context.Context, io.Reader, string, ...string) error
}
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	output := &limitedBuffer{limit: 1 << 20}
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	return output.Bytes(), err
}

type limitedBuffer struct {
	data  []byte
	limit int
	mu    sync.Mutex
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) < b.limit {
		remain := b.limit - len(b.data)
		if remain > len(value) {
			remain = len(value)
		}
		b.data = append(b.data, value[:remain]...)
	}
	return len(value), nil
}
func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data...)
}
func (ExecRunner) RunIO(ctx context.Context, in io.Reader, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = in
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

type Store struct {
	root   string
	db     *outpost.Store
	runner Runner
	podman string
	mu     sync.Mutex
}

func New(root string, db *outpost.Store, runner Runner) (*Store, error) {
	if runner == nil {
		runner = ExecRunner{}
	}
	if !filepath.IsAbs(root) || db == nil {
		return nil, errors.New("image store configuration is invalid")
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	store := &Store{root: root, db: db, runner: runner, podman: "podman"}
	if err := store.reconcile(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}
func (s *Store) imagePath(d string) string {
	return filepath.Join(s.root, strings.TrimPrefix(d, "sha256:"), "rootfs.ext4")
}
func (s *Store) reconcile(ctx context.Context) error {
	images, err := s.db.ListImages(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(images))
	for _, image := range images {
		if outpost.ValidDigest(image.Digest) {
			known[strings.TrimPrefix(image.Digest, "sha256:")] = true
		}
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(s.root, name)
		if known[name] {
			continue
		}
		if strings.HasPrefix(name, ".rootfs-") || strings.HasPrefix(name, ".build-") {
			s.cleanupExtracted(path)
			continue
		}
		for _, prefix := range []string{".context-", ".export-", ".flattened-", ".oci-", ".ext4-", ".publish-", ".debugfs-"} {
			if strings.HasPrefix(name, prefix) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
				break
			}
		}
		if len(name) == 64 {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Import(ctx context.Context, input io.Reader, tag string) (outpost.Image, error) {
	if !outpost.ValidImageTag(tag) {
		return outpost.Image{}, outpost.ErrInvalidImage
	}
	archive, err := s.copyInput(ctx, input, maxArchiveBytes, ".oci-")
	if err != nil {
		return outpost.Image{}, err
	}
	defer os.Remove(archive)
	output, err := s.run(ctx, s.podman, "load", "--input", archive)
	if err != nil {
		return outpost.Image{}, fmt.Errorf("podman load: %w", err)
	}
	id, err := loadedImageID(string(output))
	if err != nil {
		return outpost.Image{}, err
	}
	return s.export(ctx, id, tag)
}

func loadedImageID(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"Loaded image ID: ", "Loaded image: "} {
			if value, ok := strings.CutPrefix(line, prefix); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value), nil
			}
		}
	}
	return "", errors.New("podman did not report a loaded image")
}

func (s *Store) Build(ctx context.Context, input io.Reader, tag string) (outpost.Image, error) {
	if !outpost.ValidImageTag(tag) {
		return outpost.Image{}, outpost.ErrInvalidImage
	}
	dir, err := s.context(ctx, input)
	if err != nil {
		return outpost.Image{}, err
	}
	defer s.cleanupExtracted(dir)
	name := "outpost-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(int64(os.Getpid()), 36)
	if _, err := s.runFor(ctx, 30*time.Minute, s.podman, "build", "--pull=never", "--tag", name, dir); err != nil {
		return outpost.Image{}, fmt.Errorf("podman build: %w", err)
	}
	defer s.cleanup(s.podman, "image", "rm", "--force", name)
	return s.export(ctx, name, tag)
}

func (s *Store) export(ctx context.Context, source, tag string) (outpost.Image, error) {
	containerOut, err := s.run(ctx, s.podman, "create", source)
	if err != nil {
		return outpost.Image{}, fmt.Errorf("podman create: %w", err)
	}
	container := strings.TrimSpace(string(containerOut))
	if container == "" || strings.ContainsAny(container, " \t\r\n") {
		return outpost.Image{}, errors.New("podman returned an invalid container ID")
	}
	defer s.cleanup(s.podman, "rm", "--force", container)
	archive, err := os.CreateTemp(s.root, ".export-")
	if err != nil {
		return outpost.Image{}, err
	}
	archiveName := archive.Name()
	if err := archive.Close(); err != nil {
		os.Remove(archiveName)
		return outpost.Image{}, err
	}
	defer os.Remove(archiveName)
	if _, err := s.run(ctx, s.podman, "export", "--output", archiveName, container); err != nil {
		return outpost.Image{}, fmt.Errorf("podman export: %w", err)
	}
	input, err := os.Open(archiveName)
	if err != nil {
		return outpost.Image{}, err
	}
	rootfs, err := s.convert(ctx, input)
	closeErr := input.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return outpost.Image{}, err
	}
	defer os.Remove(rootfs)
	file, err := os.Open(rootfs)
	if err != nil {
		return outpost.Image{}, err
	}
	digest, size, err := s.publish(file)
	closeErr = file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return outpost.Image{}, err
	}
	if err = s.db.PutImage(ctx, digest, size, tag); err != nil {
		return outpost.Image{}, err
	}
	return s.db.GetImage(ctx, digest)
}

func (s *Store) convert(ctx context.Context, input io.Reader) (string, error) {
	archive, err := s.copyInput(ctx, input, maxArchiveBytes, ".flattened-")
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)
	if err := validateArchive(archive, maxArchiveBytes); err != nil {
		return "", err
	}
	root, err := os.MkdirTemp(s.root, ".rootfs-")
	if err != nil {
		return "", err
	}
	defer s.cleanupExtracted(root)
	if _, err := s.run(ctx, s.podman, "unshare", "tar", "--numeric-owner", "--same-owner", "--same-permissions", "-C", root, "-xf", archive); err != nil {
		return "", fmt.Errorf("extract rootfs: %w", err)
	}
	if err := s.scrub(ctx, root); err != nil {
		return "", err
	}
	if err := s.bootContract(ctx, root); err != nil {
		return "", err
	}
	used, seed, err := archiveMetadata(archive)
	if err != nil {
		return "", err
	}
	size, err := rootFSSize(used)
	if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}
	if _, err := s.run(ctx, s.podman, "unshare", "find", root, "-exec", "touch", "-h", "-d", "@0", "{}", "+"); err != nil {
		return "", err
	}
	image, err := os.CreateTemp(s.root, ".ext4-")
	if err != nil {
		return "", err
	}
	name := image.Name()
	if err := image.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if _, err := s.run(ctx, s.podman, "unshare", "truncate", "-s", strconv.FormatInt(size, 10), name); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("allocate rootfs: %w", err)
	}
	uuid := seedUUID(seed)
	args := []string{"unshare", "env", "E2FSPROGS_FAKE_TIME=1", "mkfs.ext4", "-q", "-F", "-d", root, "-O", "has_journal,^orphan_file,^metadata_csum,^metadata_csum_seed,^resize_inode", "-E", "lazy_itable_init=0,lazy_journal_init=0,hash_seed=" + uuid, "-U", uuid, name}
	if _, err := s.run(ctx, s.podman, args...); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("make rootfs: %w", err)
	}
	if err := s.e2fsck(ctx, name); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := s.normalizeExt4(ctx, name); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := s.debugfsContract(ctx, name); err != nil {
		os.Remove(name)
		return "", err
	}
	if _, err := s.run(ctx, "e2fsck", "-fn", name); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func rootFSSize(used int64) (int64, error) {
	if used < 0 || used > (maxRootFSBytes-(256<<20))*2/3 {
		return 0, errors.New("rootfs exceeds maximum size")
	}
	size := used + used/2 + 256<<20
	if size < minRootFSBytes {
		return minRootFSBytes, nil
	}
	return size, nil
}
func (s *Store) e2fsck(ctx context.Context, path string) error {
	_, err := s.run(ctx, "e2fsck", "-fy", path)
	return err
}
func (s *Store) normalizeExt4(ctx context.Context, image string) error {
	out, err := s.run(ctx, "tune2fs", "-l", image)
	if err != nil {
		return err
	}
	var count int
	for _, line := range strings.Split(string(out), "\n") {
		if value, ok := strings.CutPrefix(line, "Inode count:"); ok {
			count, err = strconv.Atoi(strings.TrimSpace(value))
			break
		}
	}
	if err != nil || count < 1 {
		return errors.New("read ext4 inode count")
	}
	commands, err := os.CreateTemp(s.root, ".debugfs-")
	if err != nil {
		return err
	}
	commandName := commands.Name()
	defer os.Remove(commandName)
	for inode := 1; inode <= count; inode++ {
		for _, field := range []string{"atime", "ctime", "mtime", "crtime"} {
			if _, err = fmt.Fprintf(commands, "set_inode_field <%d> %s 1\n", inode, field); err != nil {
				commands.Close()
				return err
			}
		}
	}
	for _, field := range []string{"mkfs_time", "wtime", "lastcheck"} {
		if _, err = fmt.Fprintf(commands, "set_super_value %s 1\n", field); err != nil {
			commands.Close()
			return err
		}
	}
	if err = commands.Close(); err != nil {
		return err
	}
	if _, err = s.run(ctx, "env", "E2FSPROGS_FAKE_TIME=1", "debugfs", "-w", "-f", commandName, image); err != nil {
		return fmt.Errorf("normalize rootfs timestamps: %w", err)
	}
	return nil
}
func (s *Store) debugfsContract(ctx context.Context, image string) error {
	resolve := func(name string) (string, string, error) {
		for depth := 0; depth < 40; depth++ {
			out, err := s.run(ctx, "debugfs", "-R", "stat "+name, image)
			text := string(out)
			if err != nil || !strings.Contains(text, "Inode:") {
				return "", "", errors.New("rootfs boot contract is incomplete")
			}
			if !strings.Contains(text, "symlink") {
				return name, text, nil
			}
			start := strings.Index(text, "Fast link dest: ")
			if start < 0 {
				return "", "", errors.New("rootfs boot contract is incomplete")
			}
			target := strings.Trim(strings.TrimSpace(text[start+len("Fast link dest: "):]), "\"")
			if strings.HasPrefix(target, "/") {
				name = path.Clean(target)
			} else {
				name = path.Clean(path.Join(path.Dir(name), target))
			}
			if name == "/" || strings.HasPrefix(name, "/../") {
				return "", "", errors.New("rootfs boot contract is incomplete")
			}
		}
		return "", "", errors.New("rootfs boot contract is incomplete")
	}
	init, stat, err := resolve("/sbin/init")
	if err != nil || !strings.Contains(stat, "regular") || !strings.Contains(init, "systemd") {
		return errors.New("rootfs boot contract is incomplete")
	}
	for _, required := range []string{"/usr/sbin/sshd", "/usr/bin/rsync", "/etc/systemd/system/multi-user.target.wants/ssh.service"} {
		_, stat, err := resolve(required)
		if err != nil || !strings.Contains(stat, "regular") {
			return errors.New("rootfs boot contract is incomplete")
		}
	}
	resolved, _, err := resolve("/etc/systemd/system/default.target")
	if err != nil || !strings.Contains(resolved, "multi-user.target") {
		return errors.New("rootfs default target is not multi-user")
	}
	return nil
}

func (s *Store) copyInput(ctx context.Context, input io.Reader, limit int64, pattern string) (string, error) {
	file, err := os.CreateTemp(s.root, pattern)
	if err != nil {
		return "", err
	}
	name := file.Name()
	defer func() {
		if err != nil {
			os.Remove(name)
		}
	}()
	n, err := copyContext(ctx, file, input, limit+1)
	if err == nil && n > limit {
		err = errors.New("image input exceeds limit")
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return name, nil
}
func copyContext(ctx context.Context, dst io.Writer, src io.Reader, limit int64) (int64, error) {
	buf := make([]byte, 128<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if total+int64(n) > limit {
				return total + int64(n), nil
			}
			wrote, writeErr := dst.Write(buf[:n])
			total += int64(wrote)
			if writeErr != nil {
				return total, writeErr
			}
			if wrote != n {
				return total, io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func validateArchive(path string, limit int64) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	tr := tar.NewReader(file)
	types := map[string]byte{}
	var total int64
	entries := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries++
		name, ok := archiveName(h.Name)
		if !ok || entries > maxEntries || h.Size < 0 || h.Size > maxFileBytes || types[name] != 0 || symlinkParent(types, name) {
			return errors.New("unsafe rootfs archive")
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeSymlink && h.Typeflag != tar.TypeLink {
			return errors.New("unsafe rootfs archive")
		}
		if h.Typeflag == tar.TypeSymlink && !safeLink(name, h.Linkname) {
			return errors.New("unsafe rootfs archive")
		}
		if h.Typeflag == tar.TypeLink {
			target, ok := archiveName(h.Linkname)
			if !ok || types[target] != tar.TypeReg {
				return errors.New("unsafe rootfs archive")
			}
		}
		types[name] = h.Typeflag
		if h.Typeflag == tar.TypeReg {
			total += h.Size
			if total > limit {
				return errors.New("rootfs archive exceeds limit")
			}
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return err
			}
		}
	}
	if entries == 0 {
		return errors.New("rootfs archive is empty")
	}
	return nil
}
func archiveName(name string) (string, bool) {
	if len(name) == 0 || len(name) > maxPathBytes || filepath.IsAbs(name) {
		return "", false
	}
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	clean := path.Clean(name)
	return clean, clean != "." && !strings.HasPrefix(clean, "../") && clean != ".."
}
func symlinkParent(types map[string]byte, name string) bool {
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		if types[parent] == tar.TypeSymlink {
			return true
		}
	}
	return false
}
func safeLink(name, target string) bool {
	if target == "" || len(target) > maxPathBytes {
		return false
	}
	if strings.HasPrefix(target, "/") {
		return true
	}
	clean := path.Clean(path.Join(path.Dir(name), target))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}
func validateContextArchive(archive string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	tr := tar.NewReader(file)
	seen := map[string]bool{}
	entries := 0
	var total int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries++
		name, ok := archiveName(h.Name)
		if !ok || seen[name] || entries > maxEntries || h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir || h.Size < 0 || h.Size > maxFileBytes {
			return errors.New("unsafe build context")
		}
		seen[name] = true
		if h.Typeflag == tar.TypeReg {
			total += h.Size
			if total > maxContextBytes {
				return errors.New("build context exceeds limit")
			}
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) context(ctx context.Context, input io.Reader) (string, error) {
	archive, err := s.copyInput(ctx, input, maxContextBytes, ".context-")
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)
	if err := validateContextArchive(archive); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(s.root, ".build-")
	if err != nil {
		return "", err
	}
	if _, err := s.run(ctx, s.podman, "unshare", "tar", "--numeric-owner", "--no-same-owner", "--no-same-permissions", "-C", dir, "-xf", archive); err != nil {
		s.cleanupExtracted(dir)
		return "", err
	}
	info, err := os.Stat(filepath.Join(dir, "Dockerfile"))
	if err != nil || !info.Mode().IsRegular() {
		os.RemoveAll(dir)
		return "", errors.New("build context requires a regular Dockerfile")
	}
	return dir, nil
}
func (s *Store) scrub(ctx context.Context, root string) error {
	paths := []string{filepath.Join(root, ".dockerenv"), filepath.Join(root, "run/.containerenv"), filepath.Join(root, "etc/machine-id"), filepath.Join(root, "var/lib/dbus/machine-id"), filepath.Join(root, "var/lib/systemd/random-seed")}
	if _, err := s.run(ctx, s.podman, append([]string{"unshare", "rm", "-f"}, paths...)...); err != nil {
		return err
	}
	if _, err := s.run(ctx, s.podman, "unshare", "find", root, "-type", "f", "-name", "authorized_keys", "-delete"); err != nil {
		return err
	}
	if _, err := s.run(ctx, s.podman, "unshare", "find", root, "-type", "f", "-path", "*/etc/ssh/ssh_host_*", "-delete"); err != nil {
		return err
	}
	return nil
}
func guestPath(root, name string) (string, error) {
	name = path.Clean("/" + name)
	result := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(name, "/")))
	rel, err := filepath.Rel(root, result)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("rootfs boot contract is incomplete")
	}
	return result, nil
}
func resolveGuest(root, name string) (string, os.FileInfo, error) {
	name = "/" + strings.TrimPrefix(name, "/")
	for depth := 0; depth < 40; depth++ {
		file, err := guestPath(root, name)
		if err != nil {
			return "", nil, err
		}
		info, err := os.Lstat(file)
		if err != nil {
			return "", nil, errors.New("rootfs boot contract is incomplete")
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return name, info, nil
		}
		target, err := os.Readlink(file)
		if err != nil {
			return "", nil, err
		}
		if strings.HasPrefix(target, "/") {
			name = target
		} else {
			name = path.Join(path.Dir(name), target)
		}
		name = path.Clean("/" + name)
	}
	return "", nil, errors.New("rootfs boot contract is incomplete")
}
func (s *Store) bootContract(ctx context.Context, root string) error {
	init, info, err := resolveGuest(root, "/sbin/init")
	if err != nil || !info.Mode().IsRegular() || !strings.Contains(init, "systemd") {
		return errors.New("rootfs boot contract is incomplete")
	}
	for _, name := range []string{"/usr/sbin/sshd", "/usr/bin/rsync", "/etc/systemd/system/multi-user.target.wants/ssh.service"} {
		_, info, err = resolveGuest(root, name)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("rootfs boot contract is incomplete")
		}
	}
	defaultTarget, _, err := resolveGuest(root, "/etc/systemd/system/default.target")
	if err != nil || !strings.Contains(defaultTarget, "multi-user.target") {
		return errors.New("rootfs default target is not multi-user")
	}
	config, err := os.ReadFile(filepath.Join(root, "etc/ssh/sshd_config"))
	text := strings.ToLower(string(config))
	if err != nil || !strings.Contains(text, "passwordauthentication no") || !(strings.Contains(text, "permitrootlogin prohibit-password") || strings.Contains(text, "permitrootlogin without-password")) {
		return errors.New("rootfs ssh configuration is incomplete")
	}
	for _, name := range []string{"etc/machine-id", "var/lib/dbus/machine-id", "root/.ssh/authorized_keys"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			return errors.New("rootfs contains an identity")
		}
	}
	return nil
}
func archiveMetadata(archive string) (int64, []byte, error) {
	file, err := os.Open(archive)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()
	tr := tar.NewReader(file)
	var used int64
	var entries [][]byte
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, nil, err
		}
		name, ok := archiveName(h.Name)
		if !ok {
			return 0, nil, errors.New("unsafe rootfs archive")
		}
		if scrubArchiveName(name) {
			continue
		}
		entry := sha256.New()
		fmt.Fprintf(entry, "%s\x00%d\x00%d\x00%o\x00%s\x00", name, h.Uid, h.Gid, h.Mode, h.Linkname)
		if h.Typeflag == tar.TypeReg {
			used += h.Size
			if _, err := io.Copy(entry, tr); err != nil {
				return 0, nil, err
			}
		}
		entries = append(entries, entry.Sum(nil))
	}
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i], entries[j]) < 0 })
	hash := sha256.New()
	for _, entry := range entries {
		hash.Write(entry)
	}
	return used, hash.Sum(nil), nil
}
func scrubArchiveName(name string) bool {
	return name == ".dockerenv" || name == "run/.containerenv" || name == "etc/machine-id" || name == "var/lib/dbus/machine-id" || name == "var/lib/systemd/random-seed" || strings.HasPrefix(name, "etc/ssh/ssh_host_") || strings.HasSuffix(name, "/.ssh/authorized_keys") || name == "root/.ssh/authorized_keys"
}

func seedUUID(seed []byte) string {
	v := append([]byte(nil), seed[:16]...)
	v[6] = v[6]&0x0f | 0x40
	v[8] = v[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", v[:4], v[4:6], v[6:8], v[8:10], v[10:16])
}
func (s *Store) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return s.runFor(ctx, 2*time.Minute, name, args...)
}
func (s *Store) runFor(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := s.runner.Run(call, name, args...)
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
func (s *Store) cleanup(name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = s.runner.Run(ctx, name, args...)
}
func (s *Store) cleanupExtracted(path string) {
	s.cleanup(s.podman, "unshare", "rm", "-rf", "--", path)
}

func (s *Store) publish(input io.Reader) (string, int64, error) {
	temp, err := os.CreateTemp(s.root, ".publish-")
	if err != nil {
		return "", 0, err
	}
	name := temp.Name()
	defer os.Remove(name)
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), input)
	if err == nil && (size == 0 || size > maxRootFSBytes) {
		err = errors.New("invalid ext4 artifact")
	}
	if err == nil {
		err = temp.Chmod(0640)
	}
	if err == nil {
		err = temp.Sync()
	}
	if err != nil {
		temp.Close()
		return "", 0, err
	}
	digest := fmt.Sprintf("sha256:%x", hash.Sum(nil))
	dir := filepath.Dir(s.imagePath(digest))
	if err := os.MkdirAll(dir, 0750); err != nil {
		temp.Close()
		return "", 0, err
	}
	if err := syncDir(s.root); err != nil {
		temp.Close()
		return "", 0, err
	}
	target := s.imagePath(digest)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Link(name, target); err != nil {
		if !errors.Is(err, os.ErrExist) {
			temp.Close()
			return "", 0, err
		}
		if err := temp.Close(); err != nil {
			return "", 0, err
		}
		if err := verifyPublished(target, digest, size); err != nil {
			return "", 0, fmt.Errorf("existing image artifact is invalid: %w", err)
		}
		return digest, size, nil
	}
	if err := temp.Close(); err != nil {
		return "", 0, err
	}
	if err := syncDir(dir); err != nil {
		return "", 0, err
	}
	if err := os.Remove(name); err != nil {
		return "", 0, err
	}
	if err := syncDir(s.root); err != nil {
		return "", 0, err
	}
	return digest, size, nil
}
func verifyPublished(path, digest string, size int64) error {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0640 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) || info.Size() != size {
		return errors.New("unsafe artifact")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("sha256:%x", hash.Sum(nil)) != digest {
		return errors.New("digest mismatch")
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Store) Remove(ctx context.Context, ref string) error {
	image, err := s.db.GetImage(ctx, ref)
	if err != nil {
		return err
	}
	if err = s.db.RemoveImage(ctx, ref); err != nil {
		return err
	}
	if outpost.ValidDigest(ref) {
		return os.RemoveAll(filepath.Dir(s.imagePath(image.Digest)))
	}
	return nil
}
func (s *Store) GC(ctx context.Context) ([]string, error) {
	ids, err := s.db.GarbageCollectImages(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := os.RemoveAll(filepath.Dir(s.imagePath(id))); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
func (s *Store) ImportDefault(ctx context.Context, archive string) error {
	canonical := "outpost/default:1"
	if _, err := s.run(ctx, s.podman, "image", "exists", canonical); err == nil {
		return nil
	}
	output, err := s.run(ctx, s.podman, "load", "--input", archive)
	if err != nil {
		return err
	}
	loaded, err := loadedImageID(string(output))
	if err != nil {
		return err
	}
	id, err := s.run(ctx, s.podman, "image", "inspect", "--format", "{{.Id}}", loaded)
	if err != nil || strings.TrimSpace(string(id)) == "" {
		return errors.New("podman did not resolve loaded default image")
	}
	if _, err = s.run(ctx, s.podman, "tag", strings.TrimSpace(string(id)), canonical); err != nil {
		return err
	}
	actual, err := s.run(ctx, s.podman, "image", "inspect", "--format", "{{.Id}}", canonical)
	if err != nil || strings.TrimSpace(string(actual)) != strings.TrimSpace(string(id)) {
		return errors.New("podman did not tag default image")
	}
	return nil
}
