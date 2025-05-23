package fileloader

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"sync/atomic"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Minimal local types for diagnostic logging to avoid import cycle
type localMinimalMutator struct {
	MutatorId      string `yaml:"mutatorid"`
	SpawnedRound   uint64 `yaml:"spawnedround,omitempty"`
	DespawnedRound uint64 `yaml:"despawnedround,omitempty"`
}
type localMinimalRoom struct {
	RoomId   int                   `yaml:"roomid"`
	Mutators []localMinimalMutator `yaml:"mutators,omitempty"`
}

type FileType uint8
type SaveOption uint8

// implements fs.ReadFileFS
// implements an iterator function as well
type ReadableGroupFS interface {
	fs.ReadFileFS
	AllFileSubSystems(yield func(fs.ReadFileFS) bool)
}

type LoadableSimple interface {
	Validate() error  // General validation (or none)
	Filepath() string // Relative file path to some base directory - can include subfolders
}

type Loadable[K comparable] interface {
	Id() K // Must be a unique identifier for the data
	LoadableSimple
}

const (
	// File types to load
	FileTypeYaml FileType = iota
	FileTypeJson

	// Save options
	SaveCareful SaveOption = iota // Save a backup and rename vs. just overwriting
)

func LoadFlatFile[T LoadableSimple](path string) (T, error) {

	var loaded T

	path = filepath.FromSlash(path)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return loaded, errors.Wrap(err, `filepath: `+path)
	}

	if fileInfo.IsDir() {
		return loaded, errors.New(`filepath: ` + path + ` is a directory`)
	}

	fpathLower := strings.ToLower(path) // Compare the whole path for extension

	if strings.HasSuffix(fpathLower, ".yaml") {

		bytes, err := os.ReadFile(path)
		if err != nil {
			return loaded, errors.Wrap(err, `filepath: `+path)
		}

		err = yaml.Unmarshal(bytes, &loaded)
		if err != nil {
			return loaded, errors.Wrap(err, `filepath: `+path)
		}

	} else if strings.HasSuffix(fpathLower, ".json") {

		bytes, err := os.ReadFile(path)
		if err != nil {
			return loaded, errors.Wrap(err, `filepath: `+path)
		}

		err = json.Unmarshal(bytes, &loaded)
		if err != nil {
			return loaded, errors.Wrap(err, `filepath: `+path)
		}

	} else {
		// Skip the file altogether
		return loaded, errors.New(`invalid file type: ` + path)
	}

	// Make sure the Filepath it claims is correct in case we need to save it later
	if !strings.HasSuffix(path, filepath.FromSlash(loaded.Filepath())) {
		return loaded, errors.New(fmt.Sprintf(`filesystem path "%s" did not end in Filepath() "%s" for type %T`, path, loaded.Filepath(), loaded))
	}

	// validate the structure
	if err := loaded.Validate(); err != nil {
		return loaded, errors.Wrap(err, `filepath: `+path)
	}

	return loaded, nil
}

// LoadAllFlatFilesSimple doesn't require a unique Id() for each item
func LoadAllFlatFilesSimple[T LoadableSimple](basePath string, fileTypes ...FileType) ([]T, error) {
	loadedData := make([]T, 0, 128)
	basePath = filepath.FromSlash(basePath)

	includeYaml := true
	includeJson := true

	if len(fileTypes) > 0 {
		includeYaml = false
		includeJson = false

		for _, fType := range fileTypes {
			if fType == FileTypeYaml {
				includeYaml = true
			} else if fType == FileTypeJson {
				includeJson = true
			}
		}
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // Skip directories
		}

		fpathLower := strings.ToLower(path)
		isFileYaml := strings.HasSuffix(fpathLower, ".yaml")
		isFileJson := strings.HasSuffix(fpathLower, ".json")

		if (!includeYaml && isFileYaml) || (!includeJson && isFileJson) {
			// This logic was slightly off, it should skip if the type is not included
			//mudlog.Warn("Skipping file due to type filter", "path", path, "includeYaml", includeYaml, "includeJson", includeJson)
			return nil // Skip if this file type is not included
		}

		if !isFileYaml && !isFileJson {
			//mudlog.Warn("Skipping file due to unknown extension", "path", path)
			return nil // Skip files that are not YAML or JSON
		}

		loaded, loadErr := LoadFlatFile[T](path)
		if loadErr != nil {
			// Check if the error is due to 'invalid file type' from LoadFlatFile, which can happen
			// if LoadFlatFile itself has stricter rules (though current logic shouldn't cause this if Walk filters correctly)
			if strings.Contains(loadErr.Error(), "invalid file type") {
				// Potentially log this as a warning or debug, as Walk should ideally prevent this.
				// For now, we'll allow it to propagate as an error from LoadAllFlatFilesSimple.
				mudlog.Warn("LoadFlatFile inside LoadAllFlatFilesSimple returned invalid file type", "path", path, "error", loadErr)
				return nil // Skip this file
			}
			return errors.Wrap(loadErr, "failed to load flat file in LoadAllFlatFilesSimple")
		}

		loadedData = append(loadedData, loaded)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return loadedData, nil
}

// Will check the ID() of each item to make sure it's unique
func LoadAllFlatFiles[K comparable, T Loadable[K]](basePath string, fileTypes ...FileType) (map[K]T, error) {
	loadedData := make(map[K]T)
	basePath = filepath.FromSlash(basePath)
	seenIds := make(map[K]string) // To track seen IDs and their file paths for duplicate error messages

	includeYaml := true
	includeJson := true

	if len(fileTypes) > 0 {
		includeYaml = false
		includeJson = false
		for _, fType := range fileTypes {
			if fType == FileTypeYaml {
				includeYaml = true
			} else if fType == FileTypeJson {
				includeJson = true
			}
		}
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil // Skip directories
		}

		fpathLower := strings.ToLower(path)
		isFileYaml := strings.HasSuffix(fpathLower, ".yaml")
		isFileJson := strings.HasSuffix(fpathLower, ".json")

		if (!includeYaml && isFileYaml) || (!includeJson && isFileJson) {
			//mudlog.Warn("Skipping file due to type filter", "path", path, "includeYaml", includeYaml, "includeJson", includeJson)
			return nil // Skip if this file type is not included
		}

		if !isFileYaml && !isFileJson {
			//mudlog.Warn("Skipping file due to unknown extension", "path", path)
			return nil // Skip files that are not YAML or JSON
		}

		loaded, loadErr := LoadFlatFile[T](path)
		if loadErr != nil {
			if strings.Contains(loadErr.Error(), "invalid file type") {
				mudlog.Warn("LoadFlatFile inside LoadAllFlatFiles returned invalid file type", "path", path, "error", loadErr)
				return nil // Skip this file
			}
			// For other errors, wrap and return
			return errors.Wrap(loadErr, fmt.Sprintf("failed to load flat file %s", path))
		}

		id := loaded.Id()
		if existingPath, ok := seenIds[id]; ok {
			return errors.New(fmt.Sprintf("duplicate ID %v found in file %s and %s", id, existingPath, path))
		}
		seenIds[id] = path
		loadedData[id] = loaded
		return nil
	})

	if err != nil {
		return nil, err
	}
	return loadedData, nil
}

// Concurrently load all files. This is just a wrapper around LoadAllFlatFiles and adds a WaitGroup.
// Returns a channel that will receive the loaded data (or nil if error) and a channel for errors.
func LoadAllFlatFilesConcurrent[K comparable, T Loadable[K]](basePath string, fileTypes ...FileType) (<-chan map[K]T, <-chan error) {
	dataChan := make(chan map[K]T, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)

		data, err := LoadAllFlatFiles[K, T](basePath, fileTypes...)
		if err != nil {
			errChan <- err
			dataChan <- nil
		} else {
			dataChan <- data
			errChan <- nil
		}
	}()

	return dataChan, errChan
}

// Enhanced SaveFlatFile to ensure directory exists and handle different save options
func SaveFlatFile[T LoadableSimple](basePath string, dataUnit T, saveOptions ...SaveOption) error {
	filePath := filepath.Join(basePath, dataUnit.Filepath())
	filePath = filepath.FromSlash(filePath)

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkDirErr := os.MkdirAll(dir, 0755); mkDirErr != nil {
			return errors.Wrap(mkDirErr, "failed to create directory for saving file: "+dir)
		}
	}

	var bytes []byte
	var err error

	fpathLower := strings.ToLower(filePath)
	if strings.HasSuffix(fpathLower, ".yaml") {
		bytes, err = yaml.Marshal(&dataUnit)
	} else if strings.HasSuffix(fpathLower, ".json") {
		bytes, err = json.MarshalIndent(&dataUnit, "", "  ") // Pretty print JSON
	} else {
		return errors.New("unsupported file type for saving: " + filePath + " (must be .yaml or .json)")
	}

	if err != nil {
		return errors.Wrap(err, "failed to marshal data for file: "+filePath)
	}

	useCarefulSave := false
	for _, opt := range saveOptions {
		if opt == SaveCareful {
			useCarefulSave = true
			break
		}
	}

	if useCarefulSave {
		backupFilePath := filePath + ".bak"
		// Check if file exists to create a backup
		if _, statErr := os.Stat(filePath); statErr == nil {
			if copyErr := CopyFileContents(filePath, backupFilePath); copyErr != nil {
				return errors.Wrap(copyErr, "failed to create backup file: "+backupFilePath)
			}
		}

		// Write to a temporary file first
		tempFilePath := filePath + ".tmp"
		if writeErr := os.WriteFile(tempFilePath, bytes, 0644); writeErr != nil {
			return errors.Wrap(writeErr, "failed to write to temporary file: "+tempFilePath)
		}

		// Rename temporary file to the target file path
		if renameErr := os.Rename(tempFilePath, filePath); renameErr != nil {
			// Attempt to restore backup if rename fails
			if _, statErr := os.Stat(backupFilePath); statErr == nil {
				if restoreErr := os.Rename(backupFilePath, filePath); restoreErr != nil {
					mudlog.Error("SaveFlatFile", "Failed to restore backup after rename failure", "backupFile", backupFilePath, "targetFile", filePath, "error", restoreErr)
				}
			}
			return errors.Wrap(renameErr, "failed to rename temporary file to target file: "+filePath)
		}
		// If successful, remove the backup file
		if _, statErr := os.Stat(backupFilePath); statErr == nil {
			os.Remove(backupFilePath) // Best effort removal
		}
	} else {
		// Simple overwrite
		if err := os.WriteFile(filePath, bytes, 0644); err != nil {
			return errors.Wrap(err, "failed to write file (overwrite): "+filePath)
		}
	}

	return nil
}

// SaveAllFlatFiles saves all data units to their respective files.
// It now uses goroutines for parallel saving, with a limit on concurrent operations.
func SaveAllFlatFiles[K comparable, T Loadable[K]](basePath string, data map[K]T, saveOptions ...SaveOption) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// Limit concurrency to avoid overwhelming the system, e.g., number of CPU cores or a fixed number.
	// runtime.NumCPU() is a sensible default.
	concurrencyLimit := runtime.NumCPU()
	if concurrencyLimit < 1 {
		concurrencyLimit = 1 // Ensure at least one worker
	}
	if concurrencyLimit > 16 { // Cap concurrency
		concurrencyLimit = 16
	}

	var wg sync.WaitGroup
	// Use a buffered channel as a semaphore to limit concurrency.
	semaphore := make(chan struct{}, concurrencyLimit)
	var errorsOccurred atomic.Value // Stores the first error encountered, if any.
	errorsOccurred.Store((error)(nil))
	var filesSaved int32 // atomic counter

	for _, dataUnit := range data {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a slot

		go func(unit T) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release slot

			// If an error has already occurred in another goroutine, skip saving this unit.
			if errorsOccurred.Load() != nil {
				return
			}

			err := SaveFlatFile(basePath, unit, saveOptions...)
			if err != nil {
				// Store the first error encountered.
				// This is a common pattern for collecting the first error in concurrent operations.
				// Subsequent errors are ignored to avoid complex error aggregation.
				currentErr := errorsOccurred.Load()
				if currentErr == nil {
					errorsOccurred.Store(err)
				}
				// Log subsequent errors if desired, but don't overwrite the first one.
				// mudlog.Error("SaveAllFlatFiles", "Error saving file", "filepath", unit.Filepath(), "error", err)
				return
			}
			atomic.AddInt32(&filesSaved, 1)
		}(dataUnit)
	}

	wg.Wait()
	close(semaphore)

	if errVal := errorsOccurred.Load(); errVal != nil {
		return int(atomic.LoadInt32(&filesSaved)), errVal.(error)
	}

	return int(atomic.LoadInt32(&filesSaved)), nil
}

// CopyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func CopyFileContents(src, dst string) (err error) {
	in, err := os.Open(filepath.FromSlash(src))
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(filepath.FromSlash(dst))
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()
	_, err = io.Copy(out, in)
	if err != nil {
		return
	}
	err = out.Sync()
	if err != nil {
		return
	}
	return
}

// GetProjectRoot returns the root directory of the project.
// It does this by looking for a go.mod file, starting from the current
// directory and going up.
func GetProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached the root of the filesystem without finding go.mod
			return "", errors.New("go.mod not found in any parent directory")
		}
		currentDir = parentDir
	}
}
