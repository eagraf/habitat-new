package web

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/rs/zerolog/log"
)

type AppDriver struct {
	webBundlePath string
}

func (d *AppDriver) Driver() string {
	return constants.AppDriverWeb
}

func (d *AppDriver) IsInstalled(pkg *node.Package, version string) (bool, error) {
	// Check for the existence of the bundle directory with the right version.
	bundleConfig, err := getWebBundleConfigFromPackage(pkg)
	if err != nil {
		return false, err
	}
	log.Info().Msgf("Installing web package %s@%s", bundleConfig.DownloadURL, version)
	bundlePath := d.getBundlePath(bundleConfig, version)

	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return false, nil
	}

	// TODO this doesn't verify the installed bundle is actually for the right application.
	// i.e. there is no guard against name conflicts right now.

	return true, nil
}

// Implement the package manager interface
func (d *AppDriver) InstallPackage(packageSpec *node.Package, version string) error {
	if packageSpec.Driver != constants.AppDriverWeb {
		return fmt.Errorf("invalid package driver: %s, expected 'web' driver", packageSpec.Driver)
	}

	// Make sure the $HABITAT_PATH/web/ directory is created
	err := os.MkdirAll(d.webBundlePath, 0755)
	if err != nil {
		return err
	}

	// Download the bundle into a temp directory.
	bundleConfig, err := getWebBundleConfigFromPackage(packageSpec)
	if err != nil {
		return err
	}

	log.Info().Msgf("Installing web package %s@%s", bundleConfig.DownloadURL, version)

	bundlePath := d.getBundlePath(bundleConfig, version)
	err = downloadAndExtractWebBundle(bundleConfig.DownloadURL, bundlePath)
	if err != nil {
		return err
	}

	return nil
}

func (d *AppDriver) UninstallPackage(pkg *node.Package, version string) error {
	bundleConfig, err := getWebBundleConfigFromPackage(pkg)
	if err != nil {
		return err
	}
	bundlePath := d.getBundlePath(bundleConfig, version)

	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(bundlePath)
}

func (d *AppDriver) getBundlePath(bundleConfig *BundleInstallationConfig, version string) string {
	return filepath.Join(d.webBundlePath, bundleConfig.BundleDirectoryName, version)
}

func getWebBundleConfigFromPackage(pkg *node.Package) (*BundleInstallationConfig, error) {
	configBytes, err := json.Marshal(pkg.DriverConfig)
	if err != nil {
		return nil, err
	}

	var bundleConfig BundleInstallationConfig
	err = json.Unmarshal(configBytes, &bundleConfig)
	if err != nil {
		return nil, err
	}

	return &bundleConfig, nil
}

func downloadAndExtractWebBundle(downloadURL string, bundlePath string) error {
	// Create a temporary directory to store the bundle
	tempDir, err := os.MkdirTemp("", "habitat-web-bundle-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Path to temporary file we download.
	tmpFile := filepath.Join(tempDir, "bundle.tar.gz")

	// Create the destination directory
	err = os.MkdirAll(bundlePath, 0755)
	if err != nil {
		return err
	}

	// Download the bundle to a temp dir.
	err = downloadWebBundle(downloadURL, tmpFile)
	if err != nil {
		return err
	}

	// Extract the bundle into the specified directory
	err = extractTarGz(tmpFile, bundlePath)
	if err != nil {
		return err
	}

	return nil
}

// Download a .tar.gz file from the specified URL.
func downloadWebBundle(downloadURL string, tmpFile string) error {
	log.Debug().Msgf("Downloading bundle from %s to %s", downloadURL, tmpFile)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create a file to save the downloaded bundle
	bundleFile, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer bundleFile.Close()

	// Copy the downloaded bundle to the file
	_, err = io.Copy(bundleFile, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func extractTarGz(tarPath, destPath string) error {
	r, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer r.Close()

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(destPath, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}
