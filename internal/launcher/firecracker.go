package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/artifact"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type LaunchProcess interface {
	Signal(os.Signal) error
	Wait() error
}

type ProcessLauncher interface {
	Start(string, []string, io.Reader, io.Writer, io.Writer) (LaunchProcess, error)
}

type OSProcessLauncher struct{}

type osLaunchProcess struct {
	process *os.Process
}

func (OSProcessLauncher) Start(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (LaunchProcess, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return osLaunchProcess{process: cmd.Process}, nil
}

func (p osLaunchProcess) Signal(signal os.Signal) error {
	return p.process.Signal(signal)
}

func (p osLaunchProcess) Wait() error {
	_, err := p.process.Wait()
	return err
}

type ProcessInfo struct {
	Exists    bool
	Verified  bool
	StartTime string
}

type ProcessController interface {
	Inspect(int, string, string) (ProcessInfo, error)
	Signal(int, os.Signal) error
}

type OSProcessController struct {
	ReadFile func(string) ([]byte, error)
	Stat     func(string) (os.FileInfo, error)
}

func (p OSProcessController) readFile(path string) ([]byte, error) {
	if p.ReadFile != nil {
		return p.ReadFile(path)
	}
	return os.ReadFile(path)
}
func (p OSProcessController) stat(path string) (os.FileInfo, error) {
	if p.Stat != nil {
		return p.Stat(path)
	}
	return os.Stat(path)
}
func (p OSProcessController) Inspect(pid int, jailRoot, executable string) (ProcessInfo, error) {
	stat, err := p.readFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if errors.Is(err, os.ErrNotExist) {
		return ProcessInfo{}, nil
	}
	if err != nil {
		return ProcessInfo{}, err
	}
	start, zombie, err := parseProcStat(stat)
	if err != nil {
		return ProcessInfo{Exists: true}, nil
	}
	if zombie {
		return ProcessInfo{}, nil
	}
	info := ProcessInfo{Exists: true, StartTime: start}
	procRoot, err := p.stat(filepath.Join("/proc", strconv.Itoa(pid), "root"))
	if errors.Is(err, os.ErrNotExist) {
		return ProcessInfo{}, nil
	}
	if err != nil {
		return info, nil
	}
	expectedRoot, err := p.stat(jailRoot)
	if err != nil || !os.SameFile(procRoot, expectedRoot) {
		return info, nil
	}
	procExecutable, err := p.stat(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if errors.Is(err, os.ErrNotExist) {
		return ProcessInfo{}, nil
	}
	if err != nil {
		return info, nil
	}
	expectedExecutable, err := p.stat(filepath.Join(jailRoot, executable))
	if err != nil || !os.SameFile(procExecutable, expectedExecutable) {
		return info, nil
	}
	info.Verified = true
	return info, nil
}

func (OSProcessController) Signal(pid int, signal os.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(signal)
}

type FirecrackerConfig struct {
	Store         *outpost.Store
	StateDir      string
	JailerBase    string
	Firecracker   string
	Jailer        string
	Kernel        string
	DefaultRootFS string
	ImageStore    string
	Uplink        string
	OutpostVMUID  int
	OutpostVMGID  int
	Runner        CommandRunner
	Launcher      ProcessLauncher
	Processes     ProcessController
	PIDTimeout    time.Duration
	StopTimeout   time.Duration
	PollInterval  time.Duration
}

type runtimeRecord struct {
	Spec      vmapi.VMSpec
	Tap       string
	GuestIP   string
	Gateway   string
	PID       int
	StartTime string
	UFWRoute  bool
}

type FirecrackerRuntime struct {
	config           FirecrackerConfig
	mu               sync.Mutex
	snapshotCapacity chan struct{}
}

type vmPaths struct {
	stateDir    string
	stateDisk   string
	log         string
	jailIDDir   string
	jailRoot    string
	jailKernel  string
	jailDisk    string
	jailConfig  string
	jailPIDFile string
}

type firecrackerConfig struct {
	BootSource        bootSource         `json:"boot-source"`
	Drives            []drive            `json:"drives"`
	MachineConfig     machineConfig      `json:"machine-config"`
	NetworkInterfaces []networkInterface `json:"network-interfaces"`
}

type bootSource struct {
	KernelImagePath string `json:"kernel_image_path"`
	BootArgs        string `json:"boot_args"`
}

type drive struct {
	DriveID    string `json:"drive_id"`
	PathOnHost string `json:"path_on_host"`
	Root       bool   `json:"is_root_device"`
	ReadOnly   bool   `json:"is_read_only"`
}

type machineConfig struct {
	VCPUs     int `json:"vcpu_count"`
	MemoryMiB int `json:"mem_size_mib"`
}

type networkInterface struct {
	ID         string `json:"iface_id"`
	HostDevice string `json:"host_dev_name"`
	MAC        string `json:"guest_mac"`
}

type processState int

const (
	processMissing processState = iota
	processVerified
	processUnverified
)

type processWait struct {
	done chan struct{}
}

var interfaceName = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,15}$`)
var executableName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,255}$`)

func NewFirecrackerRuntime(config FirecrackerConfig) (*FirecrackerRuntime, error) {
	if config.Store == nil {
		return nil, fmt.Errorf("launcher requires the shared VM database: %w", vmapi.ErrInvalid)
	}
	if config.ImageStore == "" {
		config.ImageStore = filepath.Dir(config.DefaultRootFS)
	}
	if config.Runner == nil {
		config.Runner = OSRunner{}
	}
	if config.Launcher == nil {
		config.Launcher = OSProcessLauncher{}
	}
	if config.Processes == nil {
		config.Processes = OSProcessController{}
	}
	if config.PIDTimeout == 0 {
		config.PIDTimeout = 10 * time.Second
	}
	if config.StopTimeout == 0 {
		config.StopTimeout = 5 * time.Second
	}
	if config.PollInterval == 0 {
		config.PollInterval = 50 * time.Millisecond
	}
	for _, path := range []string{config.StateDir, config.JailerBase, config.Firecracker, config.Jailer, config.Kernel, config.DefaultRootFS, config.ImageStore} {
		if !safeAbsolutePath(path) {
			return nil, fmt.Errorf("firecracker configuration: %w", vmapi.ErrInvalid)
		}
	}
	for _, path := range []string{config.Firecracker, config.Jailer, config.Kernel, config.DefaultRootFS} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || ((path == config.Firecracker || path == config.Jailer) && info.Mode()&0111 == 0) {
			return nil, fmt.Errorf("trusted asset %q: %w", path, vmapi.ErrInvalid)
		}
	}
	if !interfaceName.MatchString(config.Uplink) || config.OutpostVMUID <= 0 || config.OutpostVMGID <= 0 || config.PIDTimeout <= 0 || config.StopTimeout <= 0 || config.PollInterval <= 0 {
		return nil, fmt.Errorf("firecracker configuration: %w", vmapi.ErrInvalid)
	}
	executable := filepath.Base(config.Firecracker)
	if executable == "." || executable == string(filepath.Separator) || !executableName.MatchString(executable) {
		return nil, fmt.Errorf("firecracker executable: %w", vmapi.ErrInvalid)
	}
	// No exports can be active before this runtime is published. Remove copies
	// abandoned by a previous launcher; ordinary reconciliation must not touch
	// exports that are being copied or streamed by this runtime.
	entries, err := os.ReadDir(config.StateDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read snapshot exports: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".snapshot-") {
			if err := os.RemoveAll(filepath.Join(config.StateDir, entry.Name())); err != nil {
				return nil, fmt.Errorf("remove abandoned snapshot: %w", err)
			}
		}
	}
	return &FirecrackerRuntime{config: config, snapshotCapacity: make(chan struct{}, 1)}, nil
}

func (r *FirecrackerRuntime) rootfs(id string) (*os.File, error) {
	var file *os.File
	var err error
	if id == "default" {
		file, err = artifact.OpenFile(r.config.DefaultRootFS)
	} else {
		file, err = artifact.Open(r.config.ImageStore, id)
	}
	if err != nil {
		return nil, vmapi.ErrInvalid
	}
	return file, nil
}

func (r *FirecrackerRuntime) Create(ctx context.Context, spec vmapi.VMSpec) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := vmapi.ValidateSpec(spec); err != nil {
		return vmapi.ErrInvalid
	}
	a, err := r.config.Store.GetByID(ctx, spec.ID)
	if errors.Is(err, outpost.ErrNotFound) {
		return vmapi.ErrNotFound
	}
	if err != nil {
		return err
	}
	if runtimeView(outpost.Runtime{VM: a}).Spec != spec {
		return vmapi.ErrConflict
	}
	rootfs, err := r.rootfs(spec.ImageID)
	if err != nil {
		return vmapi.ErrInvalid
	}
	defer rootfs.Close()
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, err := r.load(spec.ID); err == nil {
		if existing.Spec != spec {
			return vmapi.ErrConflict
		}
		return nil
	} else if !errors.Is(err, vmapi.ErrNotFound) {
		return err
	}
	base, err := rootfs.Stat()
	if err != nil {
		return fmt.Errorf("stat rootfs: %w", err)
	}
	target := int64(spec.DiskGiB) << 30
	if target < base.Size() {
		return fmt.Errorf("rootfs exceeds requested disk: %w", vmapi.ErrInvalid)
	}
	paths := r.vmPaths(spec.ID)
	if err := os.MkdirAll(paths.stateDir, 0700); err != nil {
		return fmt.Errorf("create VM state: %w", err)
	}
	if err := os.Chmod(paths.stateDir, 0700); err != nil {
		return r.failCreate(paths, err)
	}
	if err := copySparse(ctx, rootfs, paths.stateDisk); err != nil {
		return r.failCreate(paths, err)
	}
	if err := os.Truncate(paths.stateDisk, target); err != nil {
		return r.failCreate(paths, err)
	}
	// Every source, including default, is opaque. Only extend the disk file;
	// filesystem growth, identity and guest configuration belong to the guest.
	if err := os.Chown(paths.stateDisk, r.config.OutpostVMUID, r.config.OutpostVMGID); err != nil {
		return r.failCreate(paths, err)
	}
	if err := os.Chmod(paths.stateDisk, 0600); err != nil {
		return r.failCreate(paths, err)
	}
	if err := r.save(spec.ID, runtimeRecord{Spec: spec}); err != nil {
		return r.failCreate(paths, err)
	}
	return nil
}

func (r *FirecrackerRuntime) failCreate(paths vmPaths, cause error) error {
	if err := os.RemoveAll(paths.stateDir); err != nil {
		return errors.Join(cause, fmt.Errorf("roll back VM state: %w", err))
	}
	return cause
}

func (r *FirecrackerRuntime) Start(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validID(id) {
		return "", vmapi.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load(id)
	if errors.Is(err, vmapi.ErrNotFound) {
		return "", vmapi.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return "", err
	}
	if state == processUnverified {
		return "", fmt.Errorf("refusing unverified live PID for %s", id)
	}
	if state == processVerified {
		if err := r.config.Store.ObserveRuntime(ctx, id, true); err != nil {
			return "", err
		}
		return m.GuestIP, nil
	}
	if m.PID != 0 {
		m.PID = 0
		m.StartTime = ""
		if err := r.save(id, m); err != nil {
			return "", err
		}
	}
	if !allocationValid(m) {
		m.Tap, m.Gateway, m.GuestIP, err = r.allocate(id)
		if err != nil {
			return "", err
		}
		if err := r.save(id, m); err != nil {
			return "", err
		}
	}
	if err := r.ensureAllocationUnique(id, m); err != nil {
		return "", err
	}
	cleanupCtx, cancel := r.cleanupContext()
	if err := r.networkDown(cleanupCtx, m); err != nil {
		cancel()
		return "", err
	}
	if err := r.removeJailID(id); err != nil {
		cancel()
		return "", err
	}
	cancel()
	m.UFWRoute = r.ufwActive(ctx)
	if err := r.save(id, m); err != nil {
		return "", err
	}
	if err := r.networkUp(ctx, m); err != nil {
		return "", err
	}
	paths := r.vmPaths(id)
	if err := r.prepareJail(paths, m); err != nil {
		return r.cleanupBeforeLaunch(id, m, err)
	}
	log, err := os.OpenFile(paths.log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return r.cleanupBeforeLaunch(id, m, err)
	}
	args := r.jailerArgs(id)
	process, err := r.config.Launcher.Start(r.config.Jailer, args, strings.NewReader(""), log, log)
	closeErr := log.Close()
	if err != nil {
		return r.cleanupBeforeLaunch(id, m, errors.Join(err, closeErr))
	}
	wait := newProcessWait(process)
	if closeErr != nil {
		return r.cleanupLaunched(id, m, process, wait, closeErr)
	}
	pid, start, err := r.waitPID(ctx, paths, wait)
	if err != nil {
		return r.cleanupLaunched(id, m, process, wait, err)
	}
	m.PID = pid
	m.StartTime = start
	if err := r.save(id, m); err != nil {
		return r.cleanupLaunched(id, m, process, wait, err)
	}
	state, err = r.runtimeProcess(m)
	if err != nil || state != processVerified {
		if err == nil {
			err = errors.New("Firecracker exited before startup completed")
		}
		return r.cleanupLaunched(id, m, process, wait, err)
	}
	if err := r.config.Store.ObserveRuntime(ctx, id, true); err != nil {
		return "", err
	}
	return m.GuestIP, nil
}

func (r *FirecrackerRuntime) prepareJail(paths vmPaths, m runtimeRecord) error {
	if err := os.MkdirAll(paths.jailRoot, 0700); err != nil {
		return err
	}
	if err := os.Chmod(paths.jailRoot, 0700); err != nil {
		return err
	}
	if err := copyFile(r.config.Kernel, paths.jailKernel, r.config.OutpostVMUID, r.config.OutpostVMGID, 0400); err != nil {
		return err
	}
	if err := os.Link(paths.stateDisk, paths.jailDisk); err != nil {
		if errors.Is(err, syscall.EXDEV) {
			return fmt.Errorf("state and jail directories must share a filesystem: %w", err)
		}
		return err
	}
	config, err := json.Marshal(firecrackerConfig{
		BootSource:        bootSource{KernelImagePath: "/vmlinux", BootArgs: "console=ttyS0 reboot=k panic=1 ip=" + m.GuestIP + "::" + m.Gateway + ":255.255.255.252::eth0:off"},
		Drives:            []drive{{DriveID: "rootfs", PathOnHost: "/rootfs.ext4", Root: true, ReadOnly: false}},
		MachineConfig:     machineConfig{VCPUs: m.Spec.VCPUs, MemoryMiB: m.Spec.MemoryMiB},
		NetworkInterfaces: []networkInterface{{ID: "eth0", HostDevice: m.Tap, MAC: mac(m.Spec.ID)}},
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(paths.jailConfig, config, 0400); err != nil {
		return err
	}
	if err := os.Chmod(paths.jailConfig, 0400); err != nil {
		return err
	}
	return os.Chown(paths.jailConfig, r.config.OutpostVMUID, r.config.OutpostVMGID)
}

func (r *FirecrackerRuntime) jailerArgs(id string) []string {
	return []string{"--id", id, "--exec-file", r.config.Firecracker, "--uid", strconv.Itoa(r.config.OutpostVMUID), "--gid", strconv.Itoa(r.config.OutpostVMGID), "--chroot-base-dir", r.config.JailerBase, "--cgroup-version", "2", "--resource-limit", "no-file=1024", "--", "--api-sock", "/firecracker.sock", "--config-file", "/config.json"}
}

func (r *FirecrackerRuntime) cleanupBeforeLaunch(id string, m runtimeRecord, cause error) (string, error) {
	ctx, cancel := r.cleanupContext()
	defer cancel()
	return "", errors.Join(cause, r.networkDown(ctx, m), r.removeJailID(id))
}

func (r *FirecrackerRuntime) cleanupLaunched(id string, m runtimeRecord, process LaunchProcess, wait *processWait, cause error) (string, error) {
	if err := r.terminateLaunched(process, wait); err != nil {
		return "", errors.Join(cause, err)
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return "", errors.Join(cause, err)
	}
	if state != processMissing {
		return "", errors.Join(cause, fmt.Errorf("preserving resources for live PID of %s", id))
	}
	m.PID = 0
	m.StartTime = ""
	if err := r.save(id, m); err != nil {
		return "", errors.Join(cause, err)
	}
	ctx, cancel := r.cleanupContext()
	defer cancel()
	return "", errors.Join(cause, r.networkDown(ctx, m), r.removeJailID(id))
}

func (r *FirecrackerRuntime) Stop(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validID(id) {
		return vmapi.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load(id)
	if errors.Is(err, vmapi.ErrNotFound) {
		return vmapi.ErrNotFound
	}
	if err != nil {
		return err
	}
	return r.stopLocked(m)
}

func (r *FirecrackerRuntime) stopLocked(m runtimeRecord) error {
	state, err := r.runtimeProcess(m)
	if err != nil {
		return err
	}
	if state == processUnverified {
		return fmt.Errorf("refusing unverified live PID for %s", m.Spec.ID)
	}
	if state == processVerified {
		if err := r.terminatePersisted(m); err != nil {
			return err
		}
	}
	m.PID = 0
	m.StartTime = ""
	if err := r.save(m.Spec.ID, m); err != nil {
		return err
	}
	ctx, cancel := r.cleanupContext()
	defer cancel()
	if err := r.networkDown(ctx, m); err != nil {
		return err
	}
	if err := r.removeJailID(m.Spec.ID); err != nil {
		return err
	}
	return r.config.Store.ObserveRuntime(ctx, m.Spec.ID, false)
}

func (r *FirecrackerRuntime) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validID(id) {
		return vmapi.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load(id)
	if errors.Is(err, vmapi.ErrNotFound) {
		if _, err := os.Lstat(r.vmPaths(id).jailIDDir); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("refusing to delete untracked VM jail for %s", id)
		}
		return os.RemoveAll(r.vmPaths(id).stateDir)
	}
	if err != nil {
		return err
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return err
	}
	if state == processUnverified {
		return fmt.Errorf("refusing unverified live PID for %s", id)
	}
	if state == processVerified {
		if err := r.terminatePersisted(m); err != nil {
			return err
		}
	}
	cleanupCtx, cancel := r.cleanupContext()
	defer cancel()
	if err := r.networkDown(cleanupCtx, m); err != nil {
		return err
	}
	if err := r.removeJailID(id); err != nil {
		return err
	}
	if err := os.RemoveAll(r.vmPaths(id).stateDir); err != nil {
		return err
	}
	return r.config.Store.RemoveRuntime(ctx, id)
}

func (r *FirecrackerRuntime) Inspect(ctx context.Context, id string) (vmapi.RuntimeState, error) {
	if err := ctx.Err(); err != nil {
		return vmapi.RuntimeState{}, err
	}
	if !validID(id) {
		return vmapi.RuntimeState{}, vmapi.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load(id)
	if errors.Is(err, vmapi.ErrNotFound) {
		if err := r.config.Store.ObserveRuntime(ctx, id, false); err != nil {
			return vmapi.RuntimeState{}, err
		}
		return vmapi.RuntimeState{}, vmapi.ErrNotFound
	}
	if err != nil {
		return vmapi.RuntimeState{}, err
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return vmapi.RuntimeState{}, err
	}
	if state == processUnverified {
		return vmapi.RuntimeState{}, fmt.Errorf("unverified live PID for %s", id)
	}
	if err := r.config.Store.ObserveRuntime(ctx, id, state == processVerified); err != nil {
		return vmapi.RuntimeState{}, err
	}
	return vmapi.RuntimeState{Spec: m.Spec, Running: state == processVerified}, nil
}

func (r *FirecrackerRuntime) List(ctx context.Context) ([]vmapi.RuntimeState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.loadAll()
	if err != nil {
		return nil, err
	}
	if err := validateUniqueAllocations(records); err != nil {
		return nil, err
	}
	out := make([]vmapi.RuntimeState, 0, len(records))
	for _, m := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state, err := r.runtimeProcess(m)
		if err != nil {
			return nil, err
		}
		if state == processUnverified {
			return nil, fmt.Errorf("unverified live PID for %s", m.Spec.ID)
		}
		if err := r.config.Store.ObserveRuntime(ctx, m.Spec.ID, state == processVerified); err != nil {
			return nil, err
		}
		out = append(out, vmapi.RuntimeState{Spec: m.Spec, Running: state == processVerified})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Spec.ID < out[j].Spec.ID })
	return out, nil
}

func (r *FirecrackerRuntime) Reconcile(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.loadAll()
	if err != nil {
		return err
	}
	if err := validateUniqueAllocations(records); err != nil {
		return err
	}
	if err := r.checkRegistry(ctx, records); err != nil {
		return err
	}
	states := make([]processState, len(records))
	for i, m := range records {
		states[i], err = r.runtimeProcess(m)
		if err != nil {
			return err
		}
		if states[i] == processUnverified {
			return fmt.Errorf("refusing unverified live PID for %s", m.Spec.ID)
		}
	}
	for i, m := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.config.Store.ObserveRuntime(ctx, m.Spec.ID, states[i] == processVerified); err != nil {
			return err
		}
		if states[i] == processVerified {
			continue
		}
		m.PID = 0
		m.StartTime = ""
		if err := r.save(m.Spec.ID, m); err != nil {
			return err
		}
		if err := r.networkDown(ctx, m); err != nil {
			return err
		}
		if err := r.removeJailID(m.Spec.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *FirecrackerRuntime) Shutdown(ctx context.Context) error {
	rows, err := r.config.Store.Runtimes(ctx)
	if err != nil {
		return err
	}
	var result error
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if err := r.Stop(ctx, row.VM.ID); err != nil && !errors.Is(err, vmapi.ErrNotFound) {
			result = errors.Join(result, fmt.Errorf("stop %s: %w", row.VM.ID, err))
		}
	}
	return result
}

func (r *FirecrackerRuntime) vmPaths(id string) vmPaths {
	stateDir := filepath.Join(r.config.StateDir, "vms", id)
	jailIDDir := filepath.Join(r.config.JailerBase, filepath.Base(r.config.Firecracker), id)
	jailRoot := filepath.Join(jailIDDir, "root")
	return vmPaths{
		stateDir:    stateDir,
		stateDisk:   filepath.Join(stateDir, "rootfs.ext4"),
		log:         filepath.Join(stateDir, "firecracker.log"),
		jailIDDir:   jailIDDir,
		jailRoot:    jailRoot,
		jailKernel:  filepath.Join(jailRoot, "vmlinux"),
		jailDisk:    filepath.Join(jailRoot, "rootfs.ext4"),
		jailConfig:  filepath.Join(jailRoot, "config.json"),
		jailPIDFile: filepath.Join(jailRoot, filepath.Base(r.config.Firecracker)+".pid"),
	}
}

func (r *FirecrackerRuntime) removeJailID(id string) error {
	return os.RemoveAll(r.vmPaths(id).jailIDDir)
}

func (r *FirecrackerRuntime) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := r.config.Runner.Run(ctx, name, args...)
	if err != nil {
		return output, fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (r *FirecrackerRuntime) allocate(id string) (string, string, string, error) {
	records, err := r.loadAll()
	if err != nil {
		return "", "", "", err
	}
	used := make(map[string]bool, len(records))
	for _, m := range records {
		if m.Spec.ID != id && allocationValid(m) {
			used[m.GuestIP] = true
		}
	}
	hash := sha256.Sum256([]byte(id))
	start := (int(hash[0])<<8 | int(hash[1])) & 16383
	for offset := 0; offset < 16384; offset++ {
		n := (start + offset) & 16383
		value := n * 4
		guest := fmt.Sprintf("172.30.%d.%d", value/256, value%256+2)
		if !used[guest] {
			return fmt.Sprintf("outpost%04x", n), fmt.Sprintf("172.30.%d.%d", value/256, value%256+1), guest, nil
		}
	}
	return "", "", "", errors.New("Outpost /30 address space is exhausted")
}

func (r *FirecrackerRuntime) ensureAllocationUnique(id string, allocation runtimeRecord) error {
	records, err := r.loadAll()
	if err != nil {
		return err
	}
	for _, m := range records {
		if m.Spec.ID != id && (m.Tap == allocation.Tap || m.Gateway == allocation.Gateway || m.GuestIP == allocation.GuestIP) {
			return fmt.Errorf("network allocation collision with %s: %w", m.Spec.ID, vmapi.ErrConflict)
		}
	}
	return nil
}

func validateUniqueAllocations(records []runtimeRecord) error {
	used := make(map[string]string)
	for _, m := range records {
		if !allocationValid(m) {
			continue
		}
		for _, value := range []string{m.Tap, m.Gateway, m.GuestIP} {
			if other := used[value]; other != "" {
				return fmt.Errorf("network allocation collision between %s and %s: %w", other, m.Spec.ID, vmapi.ErrInvalid)
			}
			used[value] = m.Spec.ID
		}
	}
	return nil
}

func (r *FirecrackerRuntime) networkUp(ctx context.Context, m runtimeRecord) error {
	if !allocationValid(m) {
		return fmt.Errorf("invalid network allocation: %w", vmapi.ErrInvalid)
	}
	commands := []struct {
		name string
		args []string
	}{
		{name: "ip", args: []string{"tuntap", "add", "dev", m.Tap, "mode", "tap", "user", strconv.Itoa(r.config.OutpostVMUID)}},
		{name: "ip", args: []string{"addr", "add", m.Gateway + "/30", "dev", m.Tap}},
		{name: "ip", args: []string{"link", "set", "dev", m.Tap, "up"}},
	}
	if m.UFWRoute {
		commands = append(commands, struct {
			name string
			args []string
		}{name: "ufw", args: []string{"route", "allow", "in", "on", m.Tap, "out", "on", r.config.Uplink}})
	}
	commands = append(commands,
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "table", "inet", networkTable(m)}},
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "chain", "inet", networkTable(m), "forward", "{", "type", "filter", "hook", "forward", "priority", "filter", ";", "policy", "accept", ";", "}"}},
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "chain", "inet", networkTable(m), "postrouting", "{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}"}},
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "rule", "inet", networkTable(m), "forward", "iifname", m.Tap, "oifname", r.config.Uplink, "accept"}},
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "rule", "inet", networkTable(m), "forward", "iifname", r.config.Uplink, "oifname", m.Tap, "ct", "state", "established,related", "accept"}},
		struct {
			name string
			args []string
		}{name: "nft", args: []string{"add", "rule", "inet", networkTable(m), "postrouting", "ip", "saddr", m.GuestIP, "oifname", r.config.Uplink, "masquerade"}},
	)
	for _, command := range commands {
		if _, err := r.run(ctx, command.name, command.args...); err != nil {
			cleanupCtx, cancel := r.cleanupContext()
			cleanupErr := r.networkDown(cleanupCtx, m)
			cancel()
			return errors.Join(err, cleanupErr)
		}
	}
	return nil
}

func (r *FirecrackerRuntime) ufwActive(ctx context.Context) bool {
	output, err := r.run(ctx, "ufw", "status")
	return err == nil && strings.HasPrefix(string(output), "Status: active")
}

func (r *FirecrackerRuntime) networkDown(ctx context.Context, m runtimeRecord) error {
	if m.Tap == "" && m.Gateway == "" && m.GuestIP == "" {
		return nil
	}
	if !allocationValid(m) {
		return fmt.Errorf("invalid network allocation: %w", vmapi.ErrInvalid)
	}
	var result error
	if m.UFWRoute {
		if _, err := r.run(ctx, "ufw", "route", "delete", "allow", "in", "on", m.Tap, "out", "on", r.config.Uplink); err != nil {
			result = errors.Join(result, err)
		}
	}
	table := networkTable(m)
	if _, err := r.run(ctx, "nft", "list", "table", "inet", table); err == nil {
		if _, err := r.run(ctx, "nft", "delete", "table", "inet", table); err != nil {
			result = errors.Join(result, err)
		}
	} else if !resourceAbsent(err) {
		result = errors.Join(result, err)
	}
	if _, err := r.run(ctx, "ip", "link", "show", "dev", m.Tap); err == nil {
		if _, err := r.run(ctx, "ip", "link", "delete", "dev", m.Tap); err != nil {
			result = errors.Join(result, err)
		}
	} else if !resourceAbsent(err) {
		result = errors.Join(result, err)
	}
	return result
}

func networkTable(m runtimeRecord) string {
	return m.Tap
}

func resourceAbsent(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such file") || strings.Contains(text, "not found") || strings.Contains(text, "does not exist") || strings.Contains(text, "cannot find device")
}

func allocationValid(m runtimeRecord) bool {
	if len(m.Tap) != 11 || !strings.HasPrefix(m.Tap, "outpost") {
		return false
	}
	guest := net.ParseIP(m.GuestIP)
	gateway := net.ParseIP(m.Gateway)
	if guest == nil || guest.To4() == nil || gateway == nil || gateway.To4() == nil {
		return false
	}
	parts := strings.Split(m.GuestIP, ".")
	if len(parts) != 4 || parts[0] != "172" || parts[1] != "30" {
		return false
	}
	third, err := strconv.Atoi(parts[2])
	if err != nil || third < 0 || third > 255 {
		return false
	}
	fourth, err := strconv.Atoi(parts[3])
	if err != nil || fourth < 2 || fourth > 254 || fourth%4 != 2 {
		return false
	}
	n := (third*256 + fourth - 2) / 4
	return m.Tap == fmt.Sprintf("outpost%04x", n) && m.Gateway == fmt.Sprintf("172.30.%d.%d", third, fourth-1)
}

func runtimeValid(m runtimeRecord) bool {
	if vmapi.ValidateSpec(m.Spec) != nil {
		return false
	}
	allocationEmpty := m.Tap == "" && m.Gateway == "" && m.GuestIP == ""
	if !allocationEmpty && !allocationValid(m) {
		return false
	}
	if (m.PID == 0) != (m.StartTime == "") || m.PID < 0 {
		return false
	}
	return m.PID == 0 || !allocationEmpty
}

func (r *FirecrackerRuntime) runtimeProcess(m runtimeRecord) (processState, error) {
	paths := r.vmPaths(m.Spec.ID)
	if m.PID > 0 {
		info, err := r.config.Processes.Inspect(m.PID, paths.jailRoot, filepath.Base(r.config.Firecracker))
		if err != nil {
			return processUnverified, err
		}
		if info.Exists && (!info.Verified || info.StartTime != m.StartTime || info.StartTime == "") {
			return processUnverified, nil
		}
		pidFile, exists, err := readPIDFile(paths.jailPIDFile)
		if err != nil {
			return processUnverified, err
		}
		if exists && pidFile != m.PID {
			other, err := r.config.Processes.Inspect(pidFile, paths.jailRoot, filepath.Base(r.config.Firecracker))
			if err != nil {
				return processUnverified, err
			}
			if other.Exists {
				return processUnverified, nil
			}
		}
		if info.Exists {
			return processVerified, nil
		}
		return processMissing, nil
	}
	pid, exists, err := readPIDFile(paths.jailPIDFile)
	if err != nil {
		return processUnverified, err
	}
	if !exists {
		return processMissing, nil
	}
	info, err := r.config.Processes.Inspect(pid, paths.jailRoot, filepath.Base(r.config.Firecracker))
	if err != nil {
		return processUnverified, err
	}
	if info.Exists {
		return processUnverified, nil
	}
	return processMissing, nil
}

func readPIDFile(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, true, fmt.Errorf("invalid Firecracker PID file")
	}
	return pid, true, nil
}

func newProcessWait(process LaunchProcess) *processWait {
	wait := &processWait{done: make(chan struct{})}
	go func() {
		_ = process.Wait()
		close(wait.done)
	}()
	return wait
}

func (r *FirecrackerRuntime) waitPID(ctx context.Context, paths vmPaths, wait *processWait) (int, string, error) {
	timeout := time.NewTimer(r.config.PIDTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()
	for {
		pid, exists, err := readPIDFile(paths.jailPIDFile)
		if err != nil {
			return 0, "", err
		}
		if exists {
			info, err := r.config.Processes.Inspect(pid, paths.jailRoot, filepath.Base(r.config.Firecracker))
			if err != nil {
				return 0, "", err
			}
			if info.Exists && !info.Verified {
				return 0, "", fmt.Errorf("unverified Firecracker PID %d", pid)
			}
			if info.Exists && info.StartTime != "" {
				return pid, info.StartTime, nil
			}
		}
		select {
		case <-ctx.Done():
			return 0, "", ctx.Err()
		case <-timeout.C:
			return 0, "", errors.New("Firecracker PID unavailable")
		case <-wait.done:
			return 0, "", errors.New("jailer exited before writing a verified Firecracker PID")
		case <-ticker.C:
		}
	}
}

func (r *FirecrackerRuntime) terminateLaunched(process LaunchProcess, wait *processWait) error {
	select {
	case <-wait.done:
		return nil
	default:
	}
	termErr := process.Signal(syscall.SIGTERM)
	if r.waitForReap(wait, r.config.StopTimeout) {
		return nil
	}
	killErr := process.Signal(syscall.SIGKILL)
	if !r.waitForReap(wait, r.config.StopTimeout) {
		return errors.Join(termErr, killErr, errors.New("launched process was not reaped"))
	}
	return nil
}

func (r *FirecrackerRuntime) waitForReap(wait *processWait, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-wait.done:
		return true
	case <-timer.C:
		return false
	}
}

func (r *FirecrackerRuntime) terminatePersisted(m runtimeRecord) error {
	if err := r.config.Processes.Signal(m.PID, syscall.SIGTERM); err != nil {
		return err
	}
	if err := r.waitStopped(m, r.config.StopTimeout); err == nil {
		return nil
	} else if !errors.Is(err, errStopTimeout) {
		return err
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return err
	}
	if state == processUnverified {
		return fmt.Errorf("PID identity changed after TERM for %s", m.Spec.ID)
	}
	if state == processMissing {
		return nil
	}
	if err := r.config.Processes.Signal(m.PID, syscall.SIGKILL); err != nil {
		return err
	}
	if err := r.waitStopped(m, r.config.StopTimeout); err != nil {
		return err
	}
	return nil
}

var errStopTimeout = errors.New("process did not stop")

func (r *FirecrackerRuntime) waitStopped(m runtimeRecord, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()
	for {
		state, err := r.runtimeProcess(m)
		if err != nil {
			return err
		}
		if state == processMissing {
			return nil
		}
		if state == processUnverified {
			return fmt.Errorf("PID identity changed while stopping %s", m.Spec.ID)
		}
		select {
		case <-timer.C:
			return errStopTimeout
		case <-ticker.C:
		}
	}
}

func (r *FirecrackerRuntime) cleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*r.config.StopTimeout)
}

func parseProcStat(data []byte) (string, bool, error) {
	close := strings.LastIndexByte(string(data), ')')
	if close < 0 || close+2 >= len(data) {
		return "", false, errors.New("invalid process stat")
	}
	fields := strings.Fields(string(data[close+2:]))
	if len(fields) < 20 {
		return "", false, errors.New("invalid process stat")
	}
	return fields[19], fields[0] == "Z", nil
}

func validID(id string) bool {
	return vmapi.ValidID(id)
}

func safeAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != string(filepath.Separator)
}

func copySparse(ctx context.Context, source *os.File, target string) (err error) {
	info, err := source.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		output.Close()
		if err != nil {
			os.Remove(target)
		}
	}()
	buffer := make([]byte, 128<<10)
	copyRange := func(start, end int64) error {
		if _, seekErr := source.Seek(start, io.SeekStart); seekErr != nil {
			return seekErr
		}
		if _, seekErr := output.Seek(start, io.SeekStart); seekErr != nil {
			return seekErr
		}
		remaining := end - start
		for remaining > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			want := int64(len(buffer))
			if remaining < want {
				want = remaining
			}
			n, readErr := source.Read(buffer[:want])
			if n > 0 {
				if _, writeErr := output.Write(buffer[:n]); writeErr != nil {
					return writeErr
				}
				remaining -= int64(n)
			}
			if readErr != nil {
				if readErr == io.EOF && remaining == 0 {
					break
				}
				return readErr
			}
		}
		return nil
	}
	for offset := int64(0); offset < info.Size(); {
		data, seekErr := syscall.Seek(int(source.Fd()), offset, 3)
		if seekErr != nil {
			if seekErr == syscall.ENXIO {
				break
			}
			if offset == 0 && (seekErr == syscall.EINVAL || seekErr == syscall.ENOTSUP) {
				return copyRange(0, info.Size())
			}
			return seekErr
		}
		hole, seekErr := syscall.Seek(int(source.Fd()), data, 4)
		if seekErr != nil {
			return seekErr
		}
		if err := copyRange(data, hole); err != nil {
			return err
		}
		offset = hole
	}
	if err = output.Truncate(info.Size()); err != nil {
		return err
	}
	if err = output.Sync(); err != nil {
		return err
	}
	return output.Close()
}

func copyFile(src, dst string, uid, gid int, mode os.FileMode) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	if err == nil {
		err = output.Sync()
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(dst, mode)
	}
	if err == nil {
		err = os.Chown(dst, uid, gid)
	}
	return err
}

func mac(id string) string {
	hash := sha256.Sum256([]byte(id))
	return fmt.Sprintf("02:fc:%02x:%02x:%02x:%02x", hash[0], hash[1], hash[2], hash[3])
}
