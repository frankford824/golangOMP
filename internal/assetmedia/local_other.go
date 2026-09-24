//go:build !linux

package assetmedia

import (
	"fmt"
	"os"
)

func FileIdentity(os.FileInfo) (string, int64, error) {
	return "", 0, fmt.Errorf("NAS media requires Linux")
}
func OpenBeneath(string, string) (*os.File, error) {
	return nil, fmt.Errorf("NAS media requires Linux")
}
func CheckLocalVersion(os.FileInfo, ReadTarget) error { return fmt.Errorf("NAS media requires Linux") }
func CloneStableSource(string, string, ReadTarget, string) error {
	return fmt.Errorf("NAS media requires Linux")
}
func AvailableBytes(string) (uint64, error) { return 0, fmt.Errorf("NAS media requires Linux") }
func AcquireWorkerWorkspace(string, string) (*os.File, error) {
	return nil, fmt.Errorf("NAS media requires Linux")
}
