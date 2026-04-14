package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
)

func archiveName(tag, goos, goarch string) string {
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("gm-%s-%s-%s%s", tag, goos, goarch, extension)
}

func extractBinary(archive []byte, goos string) ([]byte, error) {
	name := "gm"
	if goos == "windows" {
		name = "gm.exe"
		return extractZipBinary(archive, name)
	}
	return extractTarGzBinary(archive, name)
}

func extractTarGzBinary(archive []byte, binaryName string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var matches [][]byte
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			if len(matches) == 0 {
				return nil, fmt.Errorf("binary %q not found in archive", binaryName)
			}
			if len(matches) > 1 {
				return nil, fmt.Errorf("binary %q appeared multiple times in archive", binaryName)
			}
			return matches[0], nil
		case err != nil:
			return nil, fmt.Errorf("read tar.gz archive: %w", err)
		}

		if path.Base(header.Name) != binaryName {
			continue
		}
		payload, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %q from archive: %w", binaryName, err)
		}
		matches = append(matches, payload)
	}
}

func extractZipBinary(archive []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	var matches [][]byte
	for _, file := range zr.File {
		if path.Base(file.Name) != binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q in archive: %w", binaryName, err)
		}
		payload, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q from archive: %w", binaryName, err)
		}
		matches = append(matches, payload)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("binary %q not found in archive", binaryName)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("binary %q appeared multiple times in archive", binaryName)
	}
	return matches[0], nil
}
