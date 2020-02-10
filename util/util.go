package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	MaximumVolumeNameSize = 64
	parsePattern          = regexp.MustCompile(`(.*):(\d+)`)
)

const (
	BlockSizeLinux = 512
)

// ParseAddresses returns the base address and two with subsequent ports
func ParseAddresses(address string) (string, string, string, error) {
	matches := parsePattern.FindStringSubmatch(address)
	if matches == nil {
		return "", "", "", fmt.Errorf("Invalid address %s does not match pattern: %v", address, parsePattern)
	}

	host := matches[1]
	port, _ := strconv.Atoi(matches[2])

	return fmt.Sprintf("%s:%d", host, port),
		fmt.Sprintf("%s:%d", host, port+1),
		fmt.Sprintf("%s:%d", host, port+2), nil
}

func UUID() string {
	return uuid.NewV4().String()
}

func Filter(list []string, check func(string) bool) []string {
	result := make([]string, 0, len(list))
	for _, i := range list {
		if check(i) {
			result = append(result, i)
		}
	}
	return result
}

func Contains(arr []string, val string) bool {
	for _, a := range arr {
		if a == val {
			return true
		}
	}
	return false
}

type filteredLoggingHandler struct {
	filteredPaths  map[string]struct{}
	handler        http.Handler
	loggingHandler http.Handler
}

func FilteredLoggingHandler(filteredPaths map[string]struct{}, writer io.Writer, router http.Handler) http.Handler {
	return filteredLoggingHandler{
		filteredPaths:  filteredPaths,
		handler:        router,
		loggingHandler: handlers.LoggingHandler(writer, router),
	}
}

func (h filteredLoggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		if _, exists := h.filteredPaths[req.URL.Path]; exists {
			h.handler.ServeHTTP(w, req)
			return
		}
	}
	h.loggingHandler.ServeHTTP(w, req)
}

func DuplicateDevice(src, dest string) error {
	stat := unix.Stat_t{}
	if err := unix.Stat(src, &stat); err != nil {
		return fmt.Errorf("Cannot duplicate device because cannot find %s: %v", src, err)
	}
	major := int(stat.Rdev / 256)
	minor := int(stat.Rdev % 256)
	if err := mknod(dest, major, minor); err != nil {
		return fmt.Errorf("Cannot duplicate device %s to %s", src, dest)
	}
	return nil
}

func mknod(device string, major, minor int) error {
	var fileMode os.FileMode = 0600
	fileMode |= unix.S_IFBLK
	dev := int((major << 8) | (minor & 0xff) | ((minor & 0xfff00) << 12))

	logrus.Infof("Creating device %s %d:%d", device, major, minor)
	return unix.Mknod(device, uint32(fileMode), dev)
}

func RemoveDevice(dev string) error {
	var err error
	if _, err = os.Stat(dev); err == nil {
		if err = remove(dev); err != nil {
			return fmt.Errorf("Failed to removing device %s, %v", dev, err)
		}
	}
	return err
}

func removeAsync(path string, done chan<- error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logrus.Errorf("Unable to remove: %v", path)
		done <- err
	}
	done <- nil
}

func remove(path string) error {
	done := make(chan error)
	go removeAsync(path, done)
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("Timeout trying to delete %s", path)
	}
}

func ValidVolumeName(name string) bool {
	if len(name) > MaximumVolumeNameSize {
		return false
	}
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)
	return validName.MatchString(name)
}

func Volume2ISCSIName(name string) string {
	return strings.Replace(name, "_", ":", -1)
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func GetFileActualSize(file string) int64 {
	var st syscall.Stat_t
	if err := syscall.Stat(file, &st); err != nil {
		logrus.Errorf("Fail to get size of file %v", file)
		return -1
	}
	return st.Blocks * BlockSizeLinux
}

// CheckReplicationFactor returns the value of env var REPLICATION_FACTOR
// if it has not been set, then it returns 0.
func CheckReplicationFactor() int {
	replicationFactor, _ := strconv.ParseInt(os.Getenv("REPLICATION_FACTOR"), 10, 32)
	if replicationFactor == 0 {
		logrus.Infof("REPLICATION_FACTOR env not set")
		return int(replicationFactor)
	}
	return int(replicationFactor)
}

// GetReadTimeout gets the read timeout value from the env
func GetReadTimeout() time.Duration {
	readTimeout, _ := strconv.ParseInt(os.Getenv("RPC_READ_TIMEOUT"), 10, 64)
	if readTimeout == 0 {
		logrus.Infof("RPC_READ_TIMEOUT env not set")
		return time.Duration(readTimeout)
	}
	return time.Duration(readTimeout) * time.Second
}

// GetWriteTimeout gets the write timeout value from the env
func GetWriteTimeout() time.Duration {
	writeTimeout, _ := strconv.ParseInt(os.Getenv("RPC_WRITE_TIMEOUT"), 10, 64)
	if writeTimeout == 0 {
		logrus.Infof("RPC_WRITE_TIMEOUT env not set")
		return time.Duration(writeTimeout)
	}
	return time.Duration(writeTimeout) * time.Second
}
