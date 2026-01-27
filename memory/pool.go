package memory

import (
	"bytes"
	"sync"
)

// Default pool sizes.
const (
	// SmallBufferSize is the default size for small buffers (4KB).
	SmallBufferSize = 4 * 1024
	// MediumBufferSize is the default size for medium buffers (64KB).
	MediumBufferSize = 64 * 1024
	// LargeBufferSize is the default size for large buffers (1MB).
	LargeBufferSize = 1024 * 1024
)

// BufferPool manages a pool of reusable byte buffers.
type BufferPool struct {
	pool        sync.Pool
	defaultSize int
}

// NewBufferPool creates a new buffer pool with the specified default size.
func NewBufferPool(defaultSize int) *BufferPool {
	if defaultSize <= 0 {
		defaultSize = SmallBufferSize
	}
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, defaultSize))
			},
		},
		defaultSize: defaultSize,
	}
}

// Get returns a buffer from the pool.
func (p *BufferPool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	// Only return to pool if it hasn't grown too large
	if buf.Cap() <= p.defaultSize*4 {
		p.pool.Put(buf)
	}
}

// ByteSlicePool manages a pool of reusable byte slices.
type ByteSlicePool struct {
	pool        sync.Pool
	defaultSize int
}

// NewByteSlicePool creates a new byte slice pool with the specified default size.
func NewByteSlicePool(defaultSize int) *ByteSlicePool {
	if defaultSize <= 0 {
		defaultSize = SmallBufferSize
	}
	return &ByteSlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				b := make([]byte, defaultSize)
				return &b
			},
		},
		defaultSize: defaultSize,
	}
}

// Get returns a byte slice from the pool.
// The returned slice has length equal to the default size.
func (p *ByteSlicePool) Get() []byte {
	return *p.pool.Get().(*[]byte)
}

// GetWithSize returns a byte slice of at least the specified size.
// If the requested size is larger than the default, a new slice is allocated.
func (p *ByteSlicePool) GetWithSize(size int) []byte {
	if size <= p.defaultSize {
		b := p.Get()
		return b[:size]
	}
	return make([]byte, size)
}

// Put returns a byte slice to the pool.
func (p *ByteSlicePool) Put(b []byte) {
	if b == nil {
		return
	}
	// Only return to pool if it's the right size
	if cap(b) == p.defaultSize {
		p.pool.Put(&b)
	}
}

// Global pools for common use cases.
var (
	// SmallBufferPool is a global pool for small buffers (4KB).
	SmallBufferPool = NewBufferPool(SmallBufferSize)

	// MediumBufferPool is a global pool for medium buffers (64KB).
	MediumBufferPool = NewBufferPool(MediumBufferSize)

	// LargeBufferPool is a global pool for large buffers (1MB).
	LargeBufferPool = NewBufferPool(LargeBufferSize)

	// SmallBytePool is a global pool for small byte slices (4KB).
	SmallBytePool = NewByteSlicePool(SmallBufferSize)

	// MediumBytePool is a global pool for medium byte slices (64KB).
	MediumBytePool = NewByteSlicePool(MediumBufferSize)
)

// GetBuffer returns a buffer from the appropriate pool based on size hint.
func GetBuffer(sizeHint int) *bytes.Buffer {
	if sizeHint <= SmallBufferSize {
		return SmallBufferPool.Get()
	}
	if sizeHint <= MediumBufferSize {
		return MediumBufferPool.Get()
	}
	return LargeBufferPool.Get()
}

// PutBuffer returns a buffer to the appropriate pool.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	cap := buf.Cap()
	if cap <= SmallBufferSize*4 {
		SmallBufferPool.Put(buf)
	} else if cap <= MediumBufferSize*4 {
		MediumBufferPool.Put(buf)
	} else {
		LargeBufferPool.Put(buf)
	}
}

// Arena is a simple arena allocator for batch allocations.
// It pre-allocates a chunk of memory and hands out slices from it.
// This is useful for allocating many small objects that have the same lifetime.
type Arena struct {
	data   []byte
	offset int
	mu     sync.Mutex
}

// NewArena creates a new arena with the specified capacity.
func NewArena(capacity int) *Arena {
	if capacity <= 0 {
		capacity = LargeBufferSize
	}
	return &Arena{
		data: make([]byte, capacity),
	}
}

// Alloc allocates size bytes from the arena.
// Returns nil if there's not enough space.
func (a *Arena) Alloc(size int) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.offset+size > len(a.data) {
		return nil
	}

	start := a.offset
	a.offset += size
	return a.data[start:a.offset:a.offset]
}

// Reset resets the arena, allowing all memory to be reused.
func (a *Arena) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.offset = 0
}

// Used returns the number of bytes currently allocated.
func (a *Arena) Used() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.offset
}

// Capacity returns the total capacity of the arena.
func (a *Arena) Capacity() int {
	return len(a.data)
}

// Available returns the number of bytes available for allocation.
func (a *Arena) Available() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.data) - a.offset
}
