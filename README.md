# malptainer
The malptainer. This is a demo container runtime that implements a full filesystem isolation in Linux.

Written in go and purely for educational and demo purposes. Do not use for production use. Only usable in Linux as it utilises Linux syscalls.

## How to use
Please see the [How-To](./HOW-TO.md) doc for usage.

## Architecture
The tool creates isolation to provide a container through Linux syscalls. It creates a mount namespace to create filesystem isolation which is then further aided by process, network, cgroups and uts isolation.

## Not implemented
- network features are yet to be implemented.
- No image building or step-by-step installation before deployment. This means that a union flesystem like overlayfs has not been utilised to provide any sort of features.
- Only statically compiled binaries and/or shell scripts are runnable in the isolated containers. This is due to the limitation with no union based filesystem for image building.
