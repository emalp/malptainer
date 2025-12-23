package container

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// InitConfig holds the configuration passed to the init process
type InitConfig struct {
	RootfsPath  string
	ContainerDir string
	BinaryPath  string
	Hostname    string
}

// RunContainerInit is called when the binary is re-executed as the container init process
// This runs INSIDE the new namespaces and sets up the container environment
func RunContainerInit() {
	// Read config from environment variables (set by parent)
	config := InitConfig{
		RootfsPath:   os.Getenv("CNTR_ROOTFS"),
		ContainerDir: os.Getenv("CNTR_DIR"),
		BinaryPath:   os.Getenv("CNTR_BINARY"),
		Hostname:     os.Getenv("CNTR_HOSTNAME"),
	}

	if config.RootfsPath == "" {
		fatal("CNTR_ROOTFS not set")
	}

	fmt.Println("Container init: starting setup...")

	// 1. Change root mount propagation to slave recursively
	if err := unix.Mount("", "/", "", unix.MS_SLAVE|unix.MS_REC, ""); err != nil {
		fatal("failed to make root rslave: %v", err)
	}

	// 2. Recursive bind mount the rootfs to itself (required for pivot_root)
	if err := unix.Mount(config.RootfsPath, config.RootfsPath, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		fatal("failed to bind mount rootfs: %v", err)
	}

	// 3. Make the rootfs mount private
	if err := unix.Mount("", config.RootfsPath, "", unix.MS_PRIVATE, ""); err != nil {
		fatal("failed to make rootfs private: %v", err)
	}

	// 4. Create and mount /proc
	procPath := filepath.Join(config.RootfsPath, "proc")
	os.MkdirAll(procPath, 0755)
	if err := unix.Mount("proc", procPath, "proc", 0, ""); err != nil {
		fatal("failed to mount proc: %v", err)
	}

	// 5. Create and mount /dev as tmpfs
	devPath := filepath.Join(config.RootfsPath, "dev")
	os.MkdirAll(devPath, 0755)
	if err := unix.Mount("tmpfs", devPath, "tmpfs", unix.MS_NOSUID|unix.MS_STRICTATIME, "mode=0755,size=65536k"); err != nil {
		fatal("failed to mount dev tmpfs: %v", err)
	}

	// 6. Create device nodes
	createDeviceNodes(devPath)

	// 7. Create symlinks
	createDevSymlinks(devPath)

	// 8. Create and mount /dev/pts
	ptsPath := filepath.Join(devPath, "pts")
	os.MkdirAll(ptsPath, 0755)
	if err := unix.Mount("devpts", ptsPath, "devpts", 0, "newinstance,ptmxmode=0666,mode=0620"); err != nil {
		fatal("failed to mount devpts: %v", err)
	}
	os.Symlink("/dev/pts/ptmx", filepath.Join(devPath, "ptmx"))

	// 9. Create and mount /dev/mqueue
	mqueuePath := filepath.Join(devPath, "mqueue")
	os.MkdirAll(mqueuePath, 0755)
	if err := unix.Mount("mqueue", mqueuePath, "mqueue", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		// mqueue might not be available, ignore error
		fmt.Printf("Warning: failed to mount mqueue: %v\n", err)
	}

	// 10. Create and mount /dev/shm
	shmPath := filepath.Join(devPath, "shm")
	os.MkdirAll(shmPath, 0755)
	if err := unix.Mount("tmpfs", shmPath, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "mode=1777,size=67108864"); err != nil {
		fatal("failed to mount shm: %v", err)
	}

	// 11. Create and mount /sys (read-only)
	sysPath := filepath.Join(config.RootfsPath, "sys")
	os.MkdirAll(sysPath, 0755)
	if err := unix.Mount("sysfs", sysPath, "sysfs", unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		fatal("failed to mount sysfs: %v", err)
	}

	// 12. Create and mount /sys/fs/cgroup (read-only)
	cgroupPath := filepath.Join(sysPath, "fs", "cgroup")
	os.MkdirAll(cgroupPath, 0755)
	if err := unix.Mount("cgroup2", cgroupPath, "cgroup2", unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		// cgroup2 might not be available, try to continue
		fmt.Printf("Warning: failed to mount cgroup2: %v\n", err)
	}

	// 13. Bind mount /etc/hostname, /etc/hosts, /etc/resolv.conf
	bindMountNetworkFiles(config.RootfsPath, config.ContainerDir)

	// 14. Pivot root
	oldRoot := filepath.Join(config.RootfsPath, ".oldroot")
	os.MkdirAll(oldRoot, 0755)

	if err := unix.PivotRoot(config.RootfsPath, oldRoot); err != nil {
		fatal("pivot_root failed: %v", err)
	}

	// 15. Change to new root
	if err := os.Chdir("/"); err != nil {
		fatal("chdir to / failed: %v", err)
	}

	// 16. Make new root rslave
	if err := unix.Mount("", "/", "", unix.MS_SLAVE|unix.MS_REC, ""); err != nil {
		fatal("failed to make new root rslave: %v", err)
	}

	// 17. Unmount old root (lazy unmount)
	if err := unix.Unmount("/.oldroot", unix.MNT_DETACH); err != nil {
		fatal("failed to unmount old root: %v", err)
	}

	// 18. Remove old root directory
	os.RemoveAll("/.oldroot")

	// 19. Set hostname
	if config.Hostname != "" {
		if err := unix.Sethostname([]byte(config.Hostname)); err != nil {
			fmt.Printf("Warning: failed to set hostname: %v\n", err)
		}
	}

	// 20. Harden /proc - make sensitive directories read-only
	hardenProc()

	// 21. Mask sensitive paths
	maskSensitivePaths()

	fmt.Println("Container init: setup complete, executing application...")

	// 22. Finally, exec the container binary
	if err := syscall.Exec(config.BinaryPath, []string{config.BinaryPath}, os.Environ()); err != nil {
		fatal("exec failed: %v", err)
	}
}

func createDeviceNodes(devPath string) {
	// Device nodes: name, mode, major, minor
	devices := []struct {
		name  string
		mode  uint32
		major uint32
		minor uint32
	}{
		{"null", 0666, 1, 3},
		{"zero", 0666, 1, 5},
		{"full", 0666, 1, 7},
		{"random", 0666, 1, 8},
		{"urandom", 0666, 1, 9},
		{"tty", 0666, 5, 0},
	}

	for _, dev := range devices {
		path := filepath.Join(devPath, dev.name)
		devNum := unix.Mkdev(dev.major, dev.minor)
		if err := unix.Mknod(path, unix.S_IFCHR|dev.mode, int(devNum)); err != nil {
			fmt.Printf("Warning: failed to create %s: %v\n", dev.name, err)
		}
		os.Chown(path, 0, 0)
	}
}

func createDevSymlinks(devPath string) {
	symlinks := []struct {
		target string
		link   string
	}{
		{"/proc/self/fd", "fd"},
		{"/proc/self/fd/0", "stdin"},
		{"/proc/self/fd/1", "stdout"},
		{"/proc/self/fd/2", "stderr"},
		{"/proc/kcore", "core"},
	}

	for _, sl := range symlinks {
		linkPath := filepath.Join(devPath, sl.link)
		os.Symlink(sl.target, linkPath)
	}
}

func bindMountNetworkFiles(rootfsPath, containerDir string) {
	files := []string{"hostname", "hosts", "resolv.conf"}

	for _, f := range files {
		src := filepath.Join(containerDir, f)
		dst := filepath.Join(rootfsPath, "etc", f)

		// Create empty file if it doesn't exist
		file, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			file.Close()
		}

		if err := unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
			fmt.Printf("Warning: failed to bind mount %s: %v\n", f, err)
		}
	}
}

func hardenProc() {
	dirs := []string{"bus", "fs", "irq", "sys", "sysrq-trigger"}

	for _, d := range dirs {
		path := "/proc/" + d
		if _, err := os.Stat(path); err == nil {
			// Bind mount to itself, then remount read-only
			unix.Mount(path, path, "", unix.MS_BIND, "")
			unix.Mount("", path, "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY, "")
		}
	}
}

func maskSensitivePaths() {
	paths := []string{
		"/proc/asound",
		"/proc/interrupts",
		"/proc/kcore",
		"/proc/keys",
		"/proc/latency_stats",
		"/proc/timer_list",
		"/proc/timer_stats",
		"/proc/sched_debug",
		"/proc/acpi",
		"/proc/scsi",
		"/sys/firmware",
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue // Path doesn't exist
		}

		if info.IsDir() {
			// Mask directory with read-only tmpfs
			unix.Mount("tmpfs", p, "tmpfs", unix.MS_RDONLY, "")
		} else {
			// Mask file by bind mounting /dev/null
			unix.Mount("/dev/null", p, "", unix.MS_BIND, "")
		}
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Container init error: "+format+"\n", args...)
	os.Exit(1)
}

