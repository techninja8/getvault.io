package datastorage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirectory compresses the specified directory into a zip file.
func ZipDirectory(source, target string) error {
	// Ensure source path ends with separator to zip contents properly
	if !strings.HasSuffix(source, string(os.PathSeparator)) {
		source = source + string(os.PathSeparator)
	}

	// Get absolute paths to avoid any path resolution issues
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for source: %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target: %w", err)
	}

	// Create a new zip file
	zipFile, err := os.Create(absTarget)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// Create a new zip archive writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through the directory
	err = filepath.Walk(absSource, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory: %w", err)
		}

		// Calculate zip entry name from the source path
		relPath, err := filepath.Rel(absSource, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create appropriate header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %w", err)
		}

		// Set compression method and proper relative path
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		// Create zip entry
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		// For directories, we're done
		if info.IsDir() {
			return nil
		}

		// For files, copy the content
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("failed to write file content: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed while traversing directory: %w", err)
	}

	// Ensure the zip is properly closed
	if err = zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize zip file: %w", err)
	}

	// Verify the created zip
	_, err = zip.OpenReader(absTarget)
	if err != nil {
		return fmt.Errorf("created zip file verification failed: %w", err)
	}

	return nil
}

// Unzip extracts the contents of a zip file to the specified target directory.
func Unzip(source, target string) error {
	// First, let's check if the source is an actual zip file
	fileInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to access source file: %w", err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("source file is empty (0 bytes)")
	}

	// Read the first few bytes to check the ZIP signature
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}

	header := make([]byte, 4)
	_, err = file.Read(header)
	file.Close() // Close the file immediately after reading header

	if err != nil {
		return fmt.Errorf("failed to read file header: %w", err)
	}

	if string(header) != "PK\x03\x04" {
		return fmt.Errorf("not a valid zip file, missing PK header signature. Found: %x", header)
	}

	// Now open with the zip reader
	zipReader, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipReader.Close()

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(target, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Extract files
	for _, file := range zipReader.File {
		// Construct the full path for the file
		filePath := filepath.Join(target, file.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(filePath, filepath.Clean(target)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", filePath)
		}

		// Create directory tree
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create directory path if needed
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Open the file in the zip
		fileInArchive, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in archive: %w", err)
		}

		// Create the destination file
		destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileInArchive.Close()
			return fmt.Errorf("failed to create destination file: %w", err)
		}

		// Copy the contents
		_, err = io.Copy(destFile, fileInArchive)
		destFile.Close()
		fileInArchive.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}

	return nil
}

// IsValidZipFile checks if the given file is a valid ZIP archive
func IsValidZipFile(filePath string) (bool, error) {
	// Check if file exists and get its size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to access file: %w", err)
	}

	// A valid ZIP file must be at least 22 bytes (minimum size for an empty ZIP)
	if fileInfo.Size() < 22 {
		return false, fmt.Errorf("file too small to be a valid ZIP (%d bytes)", fileInfo.Size())
	}

	// Check ZIP signature (first 4 bytes should be PK\x03\x04)
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	header := make([]byte, 4)
	_, err = file.Read(header)
	if err != nil {
		return false, fmt.Errorf("failed to read file header: %w", err)
	}

	if string(header) != "PK\x03\x04" {
		return false, fmt.Errorf("invalid ZIP signature: %x", header)
	}

	// Try to actually open it as a ZIP file
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open as ZIP file: %w", err)
	}
	zipReader.Close()

	// If we got this far, it's a valid ZIP file
	return true, nil
}
