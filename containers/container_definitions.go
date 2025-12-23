package container

type Container struct {
	Name           string
	Location       string
	RootfsLocation string
	NamespacePID   int
}

var ContainersRunning = []Container{}
var ContainersStarting = []Container{}
var ContainerStopped = []Container{}
