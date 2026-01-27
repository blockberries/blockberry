package statestore

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cosmos/iavl"
)

// Snapshot errors.
var (
	ErrSnapshotNotFound      = errors.New("snapshot not found")
	ErrSnapshotExists        = errors.New("snapshot already exists")
	ErrSnapshotInvalid       = errors.New("invalid snapshot")
	ErrSnapshotChunkNotFound = errors.New("snapshot chunk not found")
	ErrSnapshotCorrupt       = errors.New("snapshot is corrupt")
)

// Default snapshot configuration.
const (
	DefaultChunkSize = 10 * 1024 * 1024 // 10 MB per chunk
	SnapshotVersion  = 1                 // Snapshot format version
)

// Snapshot represents a complete state snapshot at a specific height.
type Snapshot struct {
	// Version is the snapshot format version.
	Version uint32

	// Height is the block height this snapshot was taken at.
	Height int64

	// Hash is the unique identifier for this snapshot (sha256 of metadata+chunks).
	Hash []byte

	// ChunkSize is the maximum size of each chunk in bytes.
	ChunkSize int

	// Chunks is the total number of chunks in this snapshot.
	Chunks int

	// Metadata contains application-specific metadata.
	Metadata []byte

	// AppHash is the application hash at this height.
	AppHash []byte

	// CreatedAt is when this snapshot was created.
	CreatedAt time.Time
}

// SnapshotInfo contains summary information about a snapshot.
type SnapshotInfo struct {
	Height    int64
	Hash      []byte
	Chunks    int
	Size      int64 // Total size in bytes
	CreatedAt time.Time
}

// SnapshotChunk represents a single chunk of a snapshot.
type SnapshotChunk struct {
	// Index is the chunk index (0-based).
	Index int

	// Hash is the hash of this chunk's data.
	Hash []byte

	// Data is the chunk data.
	Data []byte
}

// SnapshotStore manages state snapshots for fast sync.
type SnapshotStore interface {
	// Create creates a snapshot at the given height.
	// The state store must be at or able to load the specified version.
	Create(height int64) (*Snapshot, error)

	// List returns information about all available snapshots.
	List() ([]*SnapshotInfo, error)

	// Load loads a snapshot by hash.
	Load(hash []byte) (*Snapshot, error)

	// LoadChunk loads a specific chunk of a snapshot.
	LoadChunk(hash []byte, index int) (*SnapshotChunk, error)

	// Delete removes a snapshot and all its chunks.
	Delete(hash []byte) error

	// Prune removes old snapshots, keeping only the most recent ones.
	Prune(keepRecent int) error

	// Has checks if a snapshot exists.
	Has(hash []byte) bool

	// Import applies a snapshot to the state store.
	// This is used during fast sync to restore state.
	Import(snapshot *Snapshot, chunkProvider ChunkProvider) error
}

// ChunkProvider provides chunks for snapshot import.
type ChunkProvider interface {
	// GetChunk returns the chunk at the given index.
	GetChunk(index int) ([]byte, error)

	// ChunkCount returns the total number of chunks.
	ChunkCount() int
}

// FileSnapshotStore implements SnapshotStore using the filesystem.
type FileSnapshotStore struct {
	path       string
	stateStore *IAVLStore
	chunkSize  int
	mu         sync.RWMutex
}

// NewFileSnapshotStore creates a new file-based snapshot store.
func NewFileSnapshotStore(path string, stateStore *IAVLStore) (*FileSnapshotStore, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	return &FileSnapshotStore{
		path:       path,
		stateStore: stateStore,
		chunkSize:  DefaultChunkSize,
	}, nil
}

// SetChunkSize sets the chunk size for new snapshots.
func (s *FileSnapshotStore) SetChunkSize(size int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunkSize = size
}

// Create creates a snapshot at the given height.
func (s *FileSnapshotStore) Create(height int64) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Export the IAVL tree at the specified version
	exporter, err := s.stateStore.tree.Export()
	if err != nil {
		return nil, fmt.Errorf("exporting iavl tree: %w", err)
	}

	// Collect all exported nodes
	var buffer bytes.Buffer
	gzWriter := gzip.NewWriter(&buffer)

	for {
		node, err := exporter.Next()
		if errors.Is(err, iavl.ErrorExportDone) {
			break
		}
		if err != nil {
			exporter.Close()
			return nil, fmt.Errorf("exporting node: %w", err)
		}

		// Encode node
		if err := encodeExportNode(gzWriter, node); err != nil {
			exporter.Close()
			return nil, fmt.Errorf("encoding node: %w", err)
		}
	}
	exporter.Close()

	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	data := buffer.Bytes()

	// Split into chunks
	chunks := splitIntoChunks(data, s.chunkSize)

	// Calculate snapshot hash
	h := sha256.New()
	h.Write(encodeInt64(height))
	for _, chunk := range chunks {
		h.Write(chunk)
	}
	hash := h.Sum(nil)

	// Create snapshot directory
	snapshotDir := filepath.Join(s.path, fmt.Sprintf("%x", hash[:8]))
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory: %w", err)
	}

	// Write chunks
	for i, chunk := range chunks {
		chunkPath := filepath.Join(snapshotDir, fmt.Sprintf("chunk_%d", i))
		if err := os.WriteFile(chunkPath, chunk, 0o644); err != nil {
			return nil, fmt.Errorf("writing chunk %d: %w", i, err)
		}
	}

	// Create snapshot
	snapshot := &Snapshot{
		Version:   SnapshotVersion,
		Height:    height,
		Hash:      hash,
		ChunkSize: s.chunkSize,
		Chunks:    len(chunks),
		AppHash:   s.stateStore.RootHash(),
		CreatedAt: time.Now(),
	}

	// Write metadata
	metadataPath := filepath.Join(snapshotDir, "metadata")
	metadata, err := encodeSnapshotMetadata(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encoding metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadata, 0o644); err != nil {
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	return snapshot, nil
}

// List returns information about all available snapshots.
func (s *FileSnapshotStore) List() ([]*SnapshotInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading snapshot directory: %w", err)
	}

	var snapshots []*SnapshotInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(s.path, entry.Name(), "metadata")
		metadata, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip invalid snapshots
		}

		snapshot, err := decodeSnapshotMetadata(metadata)
		if err != nil {
			continue
		}

		// Calculate total size
		var totalSize int64
		for i := range snapshot.Chunks {
			chunkPath := filepath.Join(s.path, entry.Name(), fmt.Sprintf("chunk_%d", i))
			info, err := os.Stat(chunkPath)
			if err == nil {
				totalSize += info.Size()
			}
		}

		snapshots = append(snapshots, &SnapshotInfo{
			Height:    snapshot.Height,
			Hash:      snapshot.Hash,
			Chunks:    snapshot.Chunks,
			Size:      totalSize,
			CreatedAt: snapshot.CreatedAt,
		})
	}

	// Sort by height descending
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Height > snapshots[j].Height
	})

	return snapshots, nil
}

// Load loads a snapshot by hash.
func (s *FileSnapshotStore) Load(hash []byte) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshotDir := filepath.Join(s.path, fmt.Sprintf("%x", hash[:8]))
	metadataPath := filepath.Join(snapshotDir, "metadata")

	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSnapshotNotFound
		}
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	snapshot, err := decodeSnapshotMetadata(metadata)
	if err != nil {
		return nil, fmt.Errorf("decoding metadata: %w", err)
	}

	// Verify hash matches
	if !bytes.Equal(snapshot.Hash, hash) {
		return nil, ErrSnapshotCorrupt
	}

	return snapshot, nil
}

// LoadChunk loads a specific chunk of a snapshot.
func (s *FileSnapshotStore) LoadChunk(hash []byte, index int) (*SnapshotChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshotDir := filepath.Join(s.path, fmt.Sprintf("%x", hash[:8]))
	chunkPath := filepath.Join(snapshotDir, fmt.Sprintf("chunk_%d", index))

	data, err := os.ReadFile(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSnapshotChunkNotFound
		}
		return nil, fmt.Errorf("reading chunk: %w", err)
	}

	chunkHash := sha256.Sum256(data)
	return &SnapshotChunk{
		Index: index,
		Hash:  chunkHash[:],
		Data:  data,
	}, nil
}

// Delete removes a snapshot and all its chunks.
func (s *FileSnapshotStore) Delete(hash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotDir := filepath.Join(s.path, fmt.Sprintf("%x", hash[:8]))
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return ErrSnapshotNotFound
	}

	return os.RemoveAll(snapshotDir)
}

// Prune removes old snapshots, keeping only the most recent ones.
func (s *FileSnapshotStore) Prune(keepRecent int) error {
	snapshots, err := s.List()
	if err != nil {
		return err
	}

	if len(snapshots) <= keepRecent {
		return nil
	}

	// Delete oldest snapshots (list is sorted by height descending)
	for _, snapshot := range snapshots[keepRecent:] {
		if err := s.Delete(snapshot.Hash); err != nil {
			return err
		}
	}

	return nil
}

// Has checks if a snapshot exists.
func (s *FileSnapshotStore) Has(hash []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshotDir := filepath.Join(s.path, fmt.Sprintf("%x", hash[:8]))
	_, err := os.Stat(snapshotDir)
	return err == nil
}

// Import applies a snapshot to the state store.
func (s *FileSnapshotStore) Import(snapshot *Snapshot, chunkProvider ChunkProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Collect all chunks
	var buffer bytes.Buffer
	for i := range chunkProvider.ChunkCount() {
		chunk, err := chunkProvider.GetChunk(i)
		if err != nil {
			return fmt.Errorf("getting chunk %d: %w", i, err)
		}
		buffer.Write(chunk)
	}

	// Decompress
	gzReader, err := gzip.NewReader(&buffer)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}

	// Import into IAVL
	importer, err := s.stateStore.tree.Import(snapshot.Height)
	if err != nil {
		return fmt.Errorf("creating importer: %w", err)
	}

	for {
		node, err := decodeExportNode(gzReader)
		if err == io.EOF {
			break
		}
		if err != nil {
			importer.Close()
			return fmt.Errorf("decoding node: %w", err)
		}

		if err := importer.Add(node); err != nil {
			importer.Close()
			return fmt.Errorf("adding node: %w", err)
		}
	}

	if err := importer.Commit(); err != nil {
		return fmt.Errorf("committing import: %w", err)
	}

	return nil
}

// Helper functions

func splitIntoChunks(data []byte, chunkSize int) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var chunks [][]byte
	for i := 0; i < len(data); i += chunkSize {
		end := min(i+chunkSize, len(data))
		chunks = append(chunks, data[i:end])
	}
	return chunks
}

func encodeInt64(v int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	return buf
}

func encodeSnapshotMetadata(s *Snapshot) ([]byte, error) {
	var buf bytes.Buffer

	// Version (4 bytes)
	if err := binary.Write(&buf, binary.BigEndian, s.Version); err != nil {
		return nil, err
	}

	// Height (8 bytes)
	if err := binary.Write(&buf, binary.BigEndian, s.Height); err != nil {
		return nil, err
	}

	// Hash length + hash
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(s.Hash))); err != nil {
		return nil, err
	}
	buf.Write(s.Hash)

	// ChunkSize (4 bytes)
	if err := binary.Write(&buf, binary.BigEndian, uint32(s.ChunkSize)); err != nil {
		return nil, err
	}

	// Chunks (4 bytes)
	if err := binary.Write(&buf, binary.BigEndian, uint32(s.Chunks)); err != nil {
		return nil, err
	}

	// AppHash length + hash
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(s.AppHash))); err != nil {
		return nil, err
	}
	buf.Write(s.AppHash)

	// CreatedAt (8 bytes)
	if err := binary.Write(&buf, binary.BigEndian, s.CreatedAt.UnixNano()); err != nil {
		return nil, err
	}

	// Metadata length + metadata
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(s.Metadata))); err != nil {
		return nil, err
	}
	buf.Write(s.Metadata)

	return buf.Bytes(), nil
}

func decodeSnapshotMetadata(data []byte) (*Snapshot, error) {
	r := bytes.NewReader(data)
	s := &Snapshot{}

	// Version
	if err := binary.Read(r, binary.BigEndian, &s.Version); err != nil {
		return nil, err
	}

	// Height
	if err := binary.Read(r, binary.BigEndian, &s.Height); err != nil {
		return nil, err
	}

	// Hash
	var hashLen uint32
	if err := binary.Read(r, binary.BigEndian, &hashLen); err != nil {
		return nil, err
	}
	s.Hash = make([]byte, hashLen)
	if _, err := io.ReadFull(r, s.Hash); err != nil {
		return nil, err
	}

	// ChunkSize
	var chunkSize uint32
	if err := binary.Read(r, binary.BigEndian, &chunkSize); err != nil {
		return nil, err
	}
	s.ChunkSize = int(chunkSize)

	// Chunks
	var chunks uint32
	if err := binary.Read(r, binary.BigEndian, &chunks); err != nil {
		return nil, err
	}
	s.Chunks = int(chunks)

	// AppHash
	var appHashLen uint32
	if err := binary.Read(r, binary.BigEndian, &appHashLen); err != nil {
		return nil, err
	}
	s.AppHash = make([]byte, appHashLen)
	if _, err := io.ReadFull(r, s.AppHash); err != nil {
		return nil, err
	}

	// CreatedAt
	var createdAt int64
	if err := binary.Read(r, binary.BigEndian, &createdAt); err != nil {
		return nil, err
	}
	s.CreatedAt = time.Unix(0, createdAt)

	// Metadata
	var metadataLen uint32
	if err := binary.Read(r, binary.BigEndian, &metadataLen); err != nil {
		return nil, err
	}
	s.Metadata = make([]byte, metadataLen)
	if _, err := io.ReadFull(r, s.Metadata); err != nil {
		return nil, err
	}

	return s, nil
}

// IAVL export node encoding
// Format: key_length (4 bytes) + key + value_length (4 bytes) + value + height (1 byte) + version (8 bytes)

func encodeExportNode(w io.Writer, node *iavl.ExportNode) error {
	// Key length + key
	if err := binary.Write(w, binary.BigEndian, uint32(len(node.Key))); err != nil {
		return err
	}
	if _, err := w.Write(node.Key); err != nil {
		return err
	}

	// Value length + value
	if err := binary.Write(w, binary.BigEndian, uint32(len(node.Value))); err != nil {
		return err
	}
	if _, err := w.Write(node.Value); err != nil {
		return err
	}

	// Height
	if err := binary.Write(w, binary.BigEndian, node.Height); err != nil {
		return err
	}

	// Version
	if err := binary.Write(w, binary.BigEndian, node.Version); err != nil {
		return err
	}

	return nil
}

func decodeExportNode(r io.Reader) (*iavl.ExportNode, error) {
	node := &iavl.ExportNode{}

	// Key length + key
	var keyLen uint32
	if err := binary.Read(r, binary.BigEndian, &keyLen); err != nil {
		return nil, err
	}
	node.Key = make([]byte, keyLen)
	if _, err := io.ReadFull(r, node.Key); err != nil {
		return nil, err
	}

	// Value length + value
	// Note: nil value indicates inner node, empty slice would fail IAVL import
	var valueLen uint32
	if err := binary.Read(r, binary.BigEndian, &valueLen); err != nil {
		return nil, err
	}
	if valueLen > 0 {
		node.Value = make([]byte, valueLen)
		if _, err := io.ReadFull(r, node.Value); err != nil {
			return nil, err
		}
	}

	// Height
	if err := binary.Read(r, binary.BigEndian, &node.Height); err != nil {
		return nil, err
	}

	// Version
	if err := binary.Read(r, binary.BigEndian, &node.Version); err != nil {
		return nil, err
	}

	return node, nil
}

// MemoryChunkProvider provides chunks from memory.
type MemoryChunkProvider struct {
	chunks [][]byte
}

// NewMemoryChunkProvider creates a new memory-based chunk provider.
func NewMemoryChunkProvider(chunks [][]byte) *MemoryChunkProvider {
	return &MemoryChunkProvider{chunks: chunks}
}

// GetChunk returns the chunk at the given index.
func (p *MemoryChunkProvider) GetChunk(index int) ([]byte, error) {
	if index < 0 || index >= len(p.chunks) {
		return nil, ErrSnapshotChunkNotFound
	}
	return p.chunks[index], nil
}

// ChunkCount returns the total number of chunks.
func (p *MemoryChunkProvider) ChunkCount() int {
	return len(p.chunks)
}

// Ensure FileSnapshotStore implements SnapshotStore.
var _ SnapshotStore = (*FileSnapshotStore)(nil)
