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
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/eagraf/habitat-new/internal/node/package_manager"
)

type WebBundleInstallationConfig struct {
	DownloadURL         string `json:"download_url"`          // Where to download the bundle from. Assume it's in a .tar.gz file.
	BundleDirectoryName string `json:"bundle_directory_name"` // The directory under $HABITAT_PATH/web/ where the bundle will be extracted into.
}

type WebDriver struct {
	PackageManager package_manager.PackageManager
}

type AppDriver struct {
	config *config.NodeConfig
}

func (d *AppDriver) Driver() string {
	return constants.AppDriverWeb
}

func (d *AppDriver) IsInstalled(packageSpec *node.Package, version string) (bool, error) {
	// Check for the existence of the bundle directory with the right version.
	bundlePath := d.getBundlePath(packageSpec, version)

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
	err := os.MkdirAll(d.config.WebBundlePath(), 0755)
	if err != nil {
		return err
	}

	// Download the bundle into a temp directory.
	bundleConfig, err := getWebBundleConfigFromPackage(packageSpec)
	if err != nil {
		return err
	}

	bundlePath := d.getBundlePath(packageSpec, version)
	err = downloadAndExtractWebBundle(bundleConfig.DownloadURL, bundlePath)
	if err != nil {
		return err
	}

	return nil
}

func (d *AppDriver) UninstallPackage(pkg *node.Package, version string) error {
	bundlePath := d.getBundlePath(pkg, version)

	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(bundlePath)
}

func (d *AppDriver) getBundlePath(pkg *node.Package, version string) string {
	return filepath.Join(d.config.WebBundlePath(), pkg.RegistryPackageID, version)
}

func NewWebDriver(config *config.NodeConfig) (*WebDriver, error) {
	return &WebDriver{
		PackageManager: &AppDriver{
			config: config,
		},
	}, nil
}

func getWebBundleConfigFromPackage(pkg *node.Package) (*WebBundleInstallationConfig, error) {
	configBytes, err := json.Marshal(pkg.DriverConfig)
	if err != nil {
		return nil, err
	}

	var bundleConfig WebBundleInstallationConfig
	err = json.Unmarshal(configBytes, &bundleConfig)
	if err != nil {
		return nil, err
	}

	return &bundleConfig, nil
}

// Download a .tar.gz file from the specified URL.
func downloadAndExtractWebBundle(downloadURL string, bundleDestPath string) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create a temporary directory to store the bundle
	tempDir, err := os.MkdirTemp("", "habitat-web-bundle-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Create a file to save the downloaded bundle
	archivePath := filepath.Join(tempDir, "bundle.tar.gz")
	bundleFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer bundleFile.Close()

	// Copy the downloaded bundle to the file
	_, err = io.Copy(bundleFile, resp.Body)
	if err != nil {
		return err
	}

	// Extract the bundle into the specified directory
	err = extractTarGz(bundleFile, bundleDestPath)
	if err != nil {
		return err
	}

	return nil
}

func extractTarGz(r io.Reader, destPath string) error {

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
