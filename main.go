package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// go run main.go run <cmd> <args> (similar to docker run)
// e.g., go run main.go run /bin/bash on a linux environment
func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("Nothing to do, guess I'll just panic...")
	}
}

func run() {
	fmt.Printf("Running parent process to spawn %v as PID %d\n", os.Args[2:], os.Getpid())

	// fork-exec current process to spawn child in an isolated environment
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// set up namespaces for process isolation
	// linux specific attributes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// hostname, process ID, mount namespaces privilege
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		// don't share new namespace with the host
		Unshareflags: syscall.CLONE_NEWNS,
	}
	// to run a rootless container use CLONE_NEWUSER flag with credentails to make the container see the non-root user as root user

	must(cmd.Run())
}

func child() {
	fmt.Printf("Running child process %v as PID %d\n", os.Args[2:], os.Getpid())

	setup_cg()

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// a container is just a linux process with a different view of the world
	must(syscall.Sethostname([]byte("container")))

	// set up a separate filesystem for the process
	must(syscall.Chroot("/home/user/dummyfs"))
	must(os.Chdir("/"))

	// the ps command gets info from /proc, so we need to mount proc in this new fs
	must(syscall.Mount("proc", "proc", "proc", 0, ""))
	// must(syscall.Mount("thing", "mytemp", "tmpfs", 0, ""))

	must(cmd.Run())

	// cleaning up
	must(syscall.Unmount("proc", 0))
}

func setup_cg() {
	// cgroups describe what resources a process can use; they are defined by pseudo dirs like /proc
	// does not work for rootless containers
	cgroups := "/sys/fs/cgroup/"

	// pids is a limit on the number of processes
	pids := filepath.Join(cgroups, "pids")
	os.Mkdir(filepath.Join(pids, "new_cgroup"), 0755)

	// max 20 processes
	must(ioutil.WriteFile(filepath.Join(pids, "new_cgroup/pids.max"), []byte("20"), 0700))

	// Removes the new cgroup in place after the container exits; if no processes left on cgroup, delete it
	must(ioutil.WriteFile(filepath.Join(pids, "new_cgroup/notify_on_release"), []byte("1"), 0700))
	must(ioutil.WriteFile(filepath.Join(pids, "new_cgroup/cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
