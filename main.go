package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	container "malptainer/containers"
)

func main() {
	// Check if we're being run as container init (re-exec pattern)
	if len(os.Args) > 1 && os.Args[1] == "init" {
		container.RunContainerInit()
		return
	}

	fmt.Println("Container Manager")
	fmt.Println("=================")

	reader := bufio.NewReader(os.Stdin)

	for {
		printMenu()

		fmt.Print("Enter choice: ")
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			// Launch a container
			fmt.Print("Enter full binary path to execute in the container (default: /bin/sh): ")
			binaryPath, _ := reader.ReadString('\n')
			binaryPath = strings.TrimSpace(binaryPath)
			if binaryPath == "" {
				binaryPath = "/bin/sh"
			}
			container.LaunchContainer(binaryPath)

		case "2":
			// List all containers
			container.ListContainers()

		case "3":
			// Delete a container
			fmt.Print("Enter container name to delete: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				fmt.Println("Container name is required")
				continue
			}
			container.DeleteContainer(name)

		case "4":
			// Shell into a container
			fmt.Print("Enter container name to shell into: ")
			name, _ := reader.ReadString('\n')
			name = strings.TrimSpace(name)
			if name == "" {
				fmt.Println("Container name is required")
				continue
			}
			container.ShellIntoContainer(name)

		case "5", "q", "Q", "exit":
			fmt.Println("Exiting...")
			container.CleanupAllContainers()
			return

		default:
			fmt.Println("Invalid choice. Please try again.")
		}

		fmt.Println()
	}
}

func printMenu() {
	fmt.Println()
	fmt.Println("Menu:")
	fmt.Println("  1. Launch a container")
	fmt.Println("  2. List all containers")
	fmt.Println("  3. Delete a container")
	fmt.Println("  4. Shell into a container")
	fmt.Println("  5. Exit")
	fmt.Println()
}
