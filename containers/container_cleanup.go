package container

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func cleanupContainers() {
	cleanupRunningContainers()
	cleanupStartingContainers()
}

// Check if a process is still running by sending signal 0
func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// Kill a process and wait until it's actually gone
func killAndWait(pid int, timeout time.Duration) error {
	// First check if process exists
	if !processExists(pid) {
		return nil // Already dead
	}

	// Try SIGTERM first
	syscall.Kill(pid, syscall.SIGTERM)

	// Wait for process to die (check every 100ms)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil // Process died
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Process didn't die, force kill with SIGKILL
	syscall.Kill(pid, syscall.SIGKILL)

	// Wait again for SIGKILL to take effect
	deadline = time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil // Process died
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Check one last time
	if processExists(pid) {
		return fmt.Errorf("process %d still exists after SIGKILL", pid)
	}

	return nil
}

func cleanupRunningContainers() {
	// Remove all running containers
	for _, container := range ContainersRunning {
		// Kill the namespace process if it exists
		if container.NamespacePID > 0 {
			fmt.Printf("Killing namespace process (PID %d) for container: %s\n", container.NamespacePID, container.Name)

			err := killAndWait(container.NamespacePID, 5*time.Second)
			if err != nil {
				fmt.Printf("Warning: %v\n", err)
			} else {
				fmt.Printf("Confirmed process %d is terminated\n", container.NamespacePID)
			}
		}

		// remove the container directory inside the .containers folder
		err := os.RemoveAll(container.Location)
		if err != nil {
			fmt.Printf("Could not remove running container: %s\n", container.Name)
		}
	}

	if len(ContainersRunning) > 0 {
		fmt.Println("Cleaned-up all running containers.")
	}
}

func cleanupStartingContainers() {
	// Remove all containers that are currently starting up..
	for _, container := range ContainersStarting {
		// Kill the namespace process if it exists
		if container.NamespacePID > 0 {
			fmt.Printf("Killing namespace process (PID %d) for container: %s\n", container.NamespacePID, container.Name)
			
			err := killAndWait(container.NamespacePID, 5*time.Second)
			if err != nil {
				fmt.Printf("Warning: %v\n", err)
			} else {
				fmt.Printf("Confirmed process %d is terminated\n", container.NamespacePID)
			}
		}

		// remove the container directory inside the .containers folder
		err := os.RemoveAll(container.Location)
		if err != nil {
			fmt.Printf("Could not remove starting container: %s\n", container.Name)
		}
	}

	if len(ContainersStarting) > 0 {
		fmt.Println("Cleaned-up all starting containers.")
	}
}
