package container

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"malptainer/utils"
	"github.com/otiai10/copy"
)

// Prepare the new container's rootfs & folders
func prepareNewContainerRootFs() Container {
	fmt.Println("Preparing root filesystem..")
	// We are enforcing the alpine rootfs for now. Otherwise, further security checks are required during the /proc mount and other steps.
	containerName := utils.GenerateRandomContainerName(7)

	// 1. First make a directory called .containers.
	// 2. Inside it make a directory with the naming convention of container-random. Keep track of the list in a list of structs.
	// All containers are deleted when the program exits for now.
	// 3. Copy the init_root_fs dir into it with the name root_fs.

	containerPath := ".containers/" + containerName
	rootFsPath := containerPath + "/root_fs"
	os.MkdirAll(rootFsPath, 0755)

	// container dir is created. Now copy the base rootfs over there
	cp_err := copy.Copy("./root_fs/", rootFsPath)
	if cp_err != nil {
		log.Fatal(cp_err)
	}

	newContainer := Container{
		Name:           containerName,
		Location:       containerPath,
		RootfsLocation: rootFsPath,
		NamespacePID:   0, // Will be set when namespaces are launched
	}

	return newContainer
}

// Prepare the temporary network files like /etc/hosts, /etc/hostname, /etc/resolv.conf
func prepareTempNetworkFiles(container Container) {
	
	// First /etc/hosts
	etcHostsContent := `127.0.0.1		localhost %s
::1				localhost ip6-localhost ip6-loopback`

	etcHostnameContent := `%s`

	etcHostsContentFormatted := fmt.Sprintf(etcHostsContent, container.Name)
	etcHostnameContentFormatted := fmt.Sprintf(etcHostnameContent, container.Name)
	err := os.WriteFile(container.Location + "/hosts", []byte(etcHostsContentFormatted), 0644)
	if err != nil {
		fmt.Printf("Could not write the /etc/hosts file temporarily")
	}

	err = os.WriteFile(container.Location + "/hostname", []byte(etcHostnameContentFormatted), 0644)
	if err != nil {
		fmt.Printf("Could not write the /etc/hostname file temporarily")
	}

	// Copy /etc/resolv.conf
	cp_err := copy.Copy("/etc/resolv.conf", container.Location + "/resolv.conf")
	if cp_err != nil {
		fmt.Printf("Could not copy /etc/resolv.conf file temporarily")
	}

}

// Launch new namespaces using the re-exec pattern (like runc)
// Creates new mount, PID, cgroup, UTS, and network namespaces, then re-execs
// the current binary as init to set up the container environment
func launchNamespaces(container *Container, binaryPath string) error {
	fmt.Println("Launching new namespaces using re-exec pattern...")

	// Copy the binary from host to container's /home/container/container-app
	containerAppDir := container.RootfsLocation + "/home/container"
	containerAppPath := containerAppDir + "/container-app"

	// Create the /home/container directory if it doesn't exist
	if err := os.MkdirAll(containerAppDir, 0755); err != nil {
		return fmt.Errorf("failed to create /home/container directory: %w", err)
	}

	// Copy the binary
	if err := copy.Copy(binaryPath, containerAppPath); err != nil {
		return fmt.Errorf("failed to copy binary to container: %w", err)
	}

	// Make it executable
	if err := os.Chmod(containerAppPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	fmt.Printf("Copied %s to container at /home/container/container-app\n", binaryPath)

	// Get absolute paths for the container
	absRootfs, err := absolutePath(container.RootfsLocation)
	if err != nil {
		return fmt.Errorf("failed to get absolute rootfs path: %w", err)
	}

	absContainerDir, err := absolutePath(container.Location)
	if err != nil {
		return fmt.Errorf("failed to get absolute container dir path: %w", err)
	}

	// Re-exec pattern: run ourselves with "init" argument
	// The child process will run RunContainerInit() which does all the setup
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |     // Mount namespace
			syscall.CLONE_NEWPID |            // PID namespace
			syscall.CLONE_NEWCGROUP |         // Cgroup namespace
			syscall.CLONE_NEWUTS |            // UTS namespace
			syscall.CLONE_NEWNET,             // Network namespace
		Setpgid: true, // Create new process group
	}

	// Pass configuration to the init process via environment variables
	cmd.Env = append(os.Environ(),
		"CNTR_ROOTFS="+absRootfs,
		"CNTR_DIR="+absContainerDir,
		"CNTR_BINARY=/home/container/container-app",
		"CNTR_HOSTNAME="+container.Name,
	)

	// Start the init process in new namespaces
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start container init process: %w", err)
	}

	// Store the PID of the namespace process
	container.NamespacePID = cmd.Process.Pid

	fmt.Printf("Launched container init process with PID %d for container: %s\n", container.NamespacePID, container.Name)

	return nil
}

// absolutePath returns the absolute path of a given path
func absolutePath(path string) (string, error) {
	if len(path) > 0 && path[0] == '/' {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd + "/" + path, nil
}
