package desktop

import (
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

func iconBytesToFilePath(iconBytes []byte) (string, error) {
	bh := md5.Sum(iconBytes)
	dataHash := hex.EncodeToString(bh[:])
	iconFilePath := filepath.Join(os.TempDir(), "systray_temp_icon_"+dataHash+".ico")

	if _, err := os.Stat(iconFilePath); os.IsNotExist(err) {
		if err := ioutil.WriteFile(iconFilePath, iconBytes, 0644); err != nil {
			return "", err
		}
	}
	return iconFilePath, nil
}

func IsHeadless() bool {
	if len(os.Getenv("SSH_CONNECTION")) > 0 {
		return true
	}
	if runtime.GOOS == "windows" {
		return false
	}
	if runtime.GOOS == "darwin" {
		return len(os.Getenv("XPC_FLAGS")) == 0
	}
	if len(os.Getenv("DISPLAY")) == 0 {
		return true
	}
	return false
}

func IsSupportTray() bool {
	if IsHeadless() {
		return false
	}
	return runtime.GOOS == "windows"
}
