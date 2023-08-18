package webviewloader

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func downloadBootstrapper() (string, error) {
	bootstrapperURL := `https://go.microsoft.com/fwlink/p/?LinkId=2124703`
	installer := filepath.Join(os.TempDir(), `MicrosoftEdgeWebview2Setup.exe`)

	// Download installer
	out, err := os.Create(installer)
	defer out.Close()
	if err != nil {
		return "", err
	}
	resp, err := http.Get(bootstrapperURL)
	if err != nil {
		err = out.Close()
		return "", err
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return installer, nil
}

func runInstaller(installer string) (bool, error) {
	// Credit: https://stackoverflow.com/a/10385867
	cmd := exec.Command(installer)
	if err := cmd.Start(); err != nil {
		return false, err
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus() == 0, nil
			}
		}
	}
	return true, nil
}

// InstallUsingBootstrapper will extract the embedded bootstrapper from Microsoft and run it to install
// the latest version of the runtime.
// Returns true if the installer ran successfully.
// Returns an error if something goes wrong
func InstallUsingBootstrapper() (bool, error) {

	installer, err := downloadBootstrapper()
	if err != nil {
		return false, err
	}

	result, err := runInstaller(installer)
	if err != nil {
		return false, err
	}

	return result, os.Remove(installer)

}
