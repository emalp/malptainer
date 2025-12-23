package container

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/term"
)

// LaunchContainer creates and starts a new container with the specified binary
func LaunchContainer(binaryPath string) {
	fmt.Printf("Launching container with binary: %s\n", binaryPath)

	// Prepare the container
	newContainer := prepareNewContainerRootFs()
	prepareTempNetworkFiles(newContainer)

	// Launch the namespaces with the binary
	err := launchNamespaces(&newContainer, binaryPath)
	if err != nil {
		fmt.Printf("Error launching container: %v\n", err)
		return
	}

	// Move container from starting to running
	ContainersRunning = append(ContainersRunning, newContainer)

	fmt.Printf("Container '%s' launched successfully (PID: %d)\n", newContainer.Name, newContainer.NamespacePID)
}

// ListContainers displays all running and starting containers
func ListContainers() {
	fmt.Println("\n=== Containers ===")

	if len(ContainersRunning) == 0 {
		fmt.Println("No containers found.")
		return
	}

	if len(ContainersRunning) > 0 {
		fmt.Println("\nRunning:")
		for _, c := range ContainersRunning {
			status := "running"
			if !processExists(c.NamespacePID) {
				status = "stopped"
			}
			fmt.Printf("  - %s (PID: %d, Status: %s)\n", c.Name, c.NamespacePID, status)
		}
	}

}

// DeleteContainer stops and removes a container by name
func DeleteContainer(name string) {
	// Search in running containers
	for i, c := range ContainersRunning {
		if c.Name == name {
			// Kill the process
			if c.NamespacePID > 0 {
				err := killAndWait(c.NamespacePID, 5*time.Second)
				if err != nil {
					fmt.Printf("Warning: %v\n", err)
				}
			}

			// Remove the container directory
			err := os.RemoveAll(c.Location)
			if err != nil {
				fmt.Printf("Error removing container directory: %v\n", err)
			}

			// Remove from list
			ContainersRunning = append(ContainersRunning[:i], ContainersRunning[i+1:]...)
			fmt.Printf("Container '%s' deleted successfully\n", name)
			return
		}
	}

	// Search in starting containers
	for i, c := range ContainersStarting {
		if c.Name == name {
			// Kill the process
			if c.NamespacePID > 0 {
				err := killAndWait(c.NamespacePID, 5*time.Second)
				if err != nil {
					fmt.Printf("Warning: %v\n", err)
				}
			}

			// Remove the container directory
			err := os.RemoveAll(c.Location)
			if err != nil {
				fmt.Printf("Error removing container directory: %v\n", err)
			}

			// Remove from list
			ContainersStarting = append(ContainersStarting[:i], ContainersStarting[i+1:]...)
			fmt.Printf("Container '%s' deleted successfully\n", name)
			return
		}
	}

	fmt.Printf("Container '%s' not found\n", name)
}

// ShellIntoContainer opens a shell in the specified container's namespaces
func ShellIntoContainer(name string) {
	var targetContainer *Container

	// Search in running containers
	for i := range ContainersRunning {
		if ContainersRunning[i].Name == name {
			targetContainer = &ContainersRunning[i]
			break
		}
	}

	if targetContainer == nil {
		fmt.Printf("Container '%s' not found\n", name)
		return
	}

	if !processExists(targetContainer.NamespacePID) {
		fmt.Printf("Container '%s' is not running (PID %d not found)\n", name, targetContainer.NamespacePID)
		return
	}

	fmt.Printf("Entering container '%s' (PID: %d)...\n", name, targetContainer.NamespacePID)

	// Save terminal state before running the shell
	fd := int(os.Stdin.Fd())
	oldState, err := term.GetState(fd)
	if err != nil {
		fmt.Printf("Warning: could not get terminal state: %v\n", err)
	}

	// Use nsenter to enter the container's namespaces
	// -F (--fork) is needed when entering PID namespace to properly become PID 1's child
	cmd := exec.Command("nsenter",
		"-t", fmt.Sprintf("%d", targetContainer.NamespacePID),
		"-m", "-u", "-n", "-C", "-p", "-F",
		"-r", "-w", // Also change root and working directory
		"/bin/sh")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	// Always restore terminal state after the shell exits
	if oldState != nil {
		term.Restore(fd, oldState)
	}

	// Also run stty sane to ensure terminal is fully reset
	resetCmd := exec.Command("stty", "sane")
	resetCmd.Stdin = os.Stdin
	resetCmd.Run()

	// Print a newline to ensure clean output
	fmt.Println()

	if err != nil {
		// Check if it's an exit error (user exited the shell)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 0 {
					return
				}
			}
		}
		fmt.Printf("Error entering container: %v\n", err)
	}
}

// CleanupAllContainers stops and removes all containers
func CleanupAllContainers() {
	fmt.Println("Cleaning up all containers...")
	cleanupContainers()
}

// CreateContainer is kept for backwards compatibility
func CreateContainer() {
	LaunchContainer("/bin/sh")
}
