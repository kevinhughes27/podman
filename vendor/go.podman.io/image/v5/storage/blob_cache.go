//go:build !containers_image_storage_stub

package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/storage"
)

// blobCacheMetadata stores metadata about a cached blob
type blobCacheMetadata struct {
	DiffID digest.Digest `json:"diff_id"`
	Size   int64         `json:"size"`
}

// blobCache provides operations for reading from and writing to a shared blob cache
type blobCache struct {
	cacheDir string
	role     string // "reader" or "writer"
	store    storage.Store
}

// newBlobCache creates a new blob cache instance from store options
func newBlobCache(store storage.Store, cacheDir, role string) *blobCache {
	if cacheDir == "" {
		logrus.Debugf("Blob cache disabled: cache directory not configured")
		return nil
	}
	// Normalize role: default to "reader" if not "writer"
	if role != "writer" {
		role = "reader"
	}
	logrus.Infof("Blob cache enabled: directory=%s role=%s", cacheDir, role)
	return &blobCache{
		cacheDir: cacheDir,
		role:     role,
		store:    store,
	}
}

// getCachePath returns the path to a cached blob file
func (bc *blobCache) getCachePath(blobDigest digest.Digest) string {
	if bc == nil || blobDigest == "" {
		return ""
	}
	return filepath.Join(bc.cacheDir, blobDigest.Encoded())
}

// getMetadataPath returns the path to the metadata file for a cached blob
func (bc *blobCache) getMetadataPath(blobDigest digest.Digest) string {
	if bc == nil || blobDigest == "" {
		return ""
	}
	return filepath.Join(bc.cacheDir, blobDigest.Encoded()+".meta")
}

// canRead returns true if the cache can be read from
func (bc *blobCache) canRead() bool {
	return bc != nil && bc.cacheDir != ""
}

// canWrite returns true if the cache can be written to
func (bc *blobCache) canWrite() bool {
	return bc != nil && bc.cacheDir != "" && bc.role == "writer"
}

// readMetadata reads the metadata for a cached blob
func (bc *blobCache) readMetadata(blobDigest digest.Digest) (*blobCacheMetadata, error) {
	if !bc.canRead() {
		return nil, fmt.Errorf("blob cache not available")
	}
	metaPath := bc.getMetadataPath(blobDigest)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var meta blobCacheMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}
	return &meta, nil
}

// writeMetadata writes metadata for a cached blob
func (bc *blobCache) writeMetadata(blobDigest digest.Digest, meta *blobCacheMetadata) error {
	if !bc.canWrite() {
		return fmt.Errorf("blob cache write not enabled")
	}
	metaPath := bc.getMetadataPath(blobDigest)
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}
	// Atomic write: write to temp file, then rename
	tempPath := metaPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	if err := os.Rename(tempPath, metaPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming metadata: %w", err)
	}
	return nil
}

// getCachedBlob checks if a blob exists in the cache and returns its path and metadata
func (bc *blobCache) getCachedBlob(blobDigest digest.Digest) (string, *blobCacheMetadata, error) {
	if !bc.canRead() {
		logrus.Debugf("Blob cache read not available for %s", blobDigest)
		return "", nil, nil
	}
	cachePath := bc.getCachePath(blobDigest)
	logrus.Debugf("Checking blob cache for %s at %s", blobDigest, cachePath)
	// Check if cache file exists
	if stat, err := os.Stat(cachePath); err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("Blob %s not found in cache (file does not exist)", blobDigest)
			return "", nil, nil
		}
		logrus.Debugf("Error checking cache file for %s: %v", blobDigest, err)
		return "", nil, fmt.Errorf("checking cache file: %w", err)
	} else if stat.Size() == 0 {
		// Empty file, treat as not found
		logrus.Debugf("Blob %s cache file is empty, treating as not found", blobDigest)
		return "", nil, nil
	}
	// Read metadata
	meta, err := bc.readMetadata(blobDigest)
	if err != nil {
		// Metadata missing or corrupted, but file exists - log and treat as not found
		logrus.Debugf("Failed to read blob cache metadata for %s: %v", blobDigest, err)
		return "", nil, nil
	}
	logrus.Debugf("Found blob %s in cache: size=%d diffID=%s", blobDigest, meta.Size, meta.DiffID)
	return cachePath, meta, nil
}

// copyOrLinkBlob copies or hard links a blob file from source to destination
// Prefers hard link (faster, same inode), falls back to copy
func copyOrLinkBlob(src, dst string) error {
	logrus.Debugf("copyOrLinkBlob: src=%s dst=%s", src, dst)
	// Try hard link first (faster, same inode)
	err := os.Link(src, dst)
	if err == nil {
		logrus.Debugf("copyOrLinkBlob: successfully hard linked %s to %s", src, dst)
		return nil
	}
	logrus.Debugf("copyOrLinkBlob: hard link failed, falling back to copy: %v", err)
	// Fall back to copy
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	copied, err := io.Copy(dstFile, srcFile)
	if err != nil {
		os.Remove(dst)
		return fmt.Errorf("copying file: %w", err)
	}
	logrus.Debugf("copyOrLinkBlob: successfully copied %d bytes from %s to %s", copied, src, dst)
	return nil
}

// saveBlobToCache saves a blob to the shared cache with atomic write
func (bc *blobCache) saveBlobToCache(srcFilePath string, blobDigest digest.Digest, diffID digest.Digest, size int64) error {
	if !bc.canWrite() {
		return fmt.Errorf("blob cache write not enabled")
	}
	// Create cache directory if needed
	if err := os.MkdirAll(bc.cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	cachePath := bc.getCachePath(blobDigest)
	// Atomic write: write to temp file, then rename
	tempPath := cachePath + ".tmp"
	if err := copyOrLinkBlob(srcFilePath, tempPath); err != nil {
		return fmt.Errorf("copying blob to cache: %w", err)
	}
	// Save metadata
	meta := &blobCacheMetadata{
		DiffID: diffID,
		Size:   size,
	}
	if err := bc.writeMetadata(blobDigest, meta); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("saving metadata: %w", err)
	}
	// Atomic rename
	if err := os.Rename(tempPath, cachePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("renaming cache file: %w", err)
	}
	logrus.Infof("Saved blob %s to cache at %s", blobDigest, cachePath)
	return nil
}
