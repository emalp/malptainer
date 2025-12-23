# How to use the tool and run containers

## Download crane & Setup Rootfs

To create a rootfs that you want your container to have, you need to download a pre-existing root filesystem of one of the distros. Typically the Alpine Linux image suffices.

Download Crane from: https://github.com/google/go-containerregistry/tree/main/cmd/crane

Then, generate your rootfs.

Inside this repo's directory:
`ROOTFS_DIR=./root_fs`
`crane export alpine:3 | sudo tar -xvC $ROOTFS_DIR`

## Compile malptainer
Run `go build .` in the repo's directory. This build the go source files and generates the `malptainer` binary.

## Run and enjoy
Run the malptainer binary and it will show you the correct menu to create, list, remove and shell into the specified containers. When you're creating a container make sure to place the absolute path of the binary you want the container to pull into itself and run it.
