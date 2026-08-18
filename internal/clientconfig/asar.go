package clientconfig

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// electronPackageVersion reads only package.json from a standard uncompressed
// Electron ASAR. It neither executes the application nor trusts an adjacent
// mutable version file.
func electronPackageVersion(path string) (string, error) {
	if err := rejectSymlinks(path); err != nil {
		return "", err
	}
	if err := ensureTrustedEvidence(path); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil || binary.LittleEndian.Uint32(header[:4]) != 4 {
		return "", errors.New("invalid Electron ASAR header")
	}
	headerSize := binary.LittleEndian.Uint32(header[4:8])
	jsonSize := binary.LittleEndian.Uint32(header[12:16])
	if headerSize < 8 || headerSize > 16<<20 || jsonSize == 0 || jsonSize > headerSize-8 {
		return "", errors.New("invalid Electron ASAR header size")
	}
	headerJSON := make([]byte, jsonSize)
	if _, err := io.ReadFull(file, headerJSON); err != nil {
		return "", err
	}
	var index struct {
		Files map[string]struct {
			Size   uint64 `json:"size"`
			Offset string `json:"offset"`
		} `json:"files"`
	}
	if err := json.Unmarshal(headerJSON, &index); err != nil {
		return "", errors.New("invalid Electron ASAR index")
	}
	entry, ok := index.Files["package.json"]
	if !ok || entry.Size == 0 || entry.Size > 1<<20 {
		return "", errors.New("Electron ASAR has no bounded package.json")
	}
	offset, err := strconv.ParseUint(entry.Offset, 10, 63)
	if err != nil {
		return "", errors.New("invalid Electron ASAR package offset")
	}
	data := make([]byte, entry.Size)
	if _, err := file.ReadAt(data, int64(8+uint64(headerSize)+offset)); err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", fmt.Errorf("invalid Electron package manifest")
	}
	return manifest.Version, nil
}
