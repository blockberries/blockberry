package memory

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBufferPool_Basic(t *testing.T) {
	pool := NewBufferPool(1024)

	// Get a buffer
	buf := pool.Get()
	require.NotNil(t, buf)
	require.Equal(t, 0, buf.Len())
	require.GreaterOrEqual(t, buf.Cap(), 1024)

	// Write to it
	buf.WriteString("hello world")
	require.Equal(t, 11, buf.Len())

	// Put it back
	pool.Put(buf)

	// Get another buffer - may or may not be the same one
	buf2 := pool.Get()
	require.NotNil(t, buf2)
	require.Equal(t, 0, buf2.Len()) // Should be reset
}

func TestBufferPool_NilPut(t *testing.T) {
	pool := NewBufferPool(1024)
	pool.Put(nil) // Should not panic
}

func TestBufferPool_DefaultSize(t *testing.T) {
	// Zero or negative size should use default
	pool := NewBufferPool(0)
	buf := pool.Get()
	require.GreaterOrEqual(t, buf.Cap(), SmallBufferSize)
	pool.Put(buf)

	pool2 := NewBufferPool(-100)
	buf2 := pool2.Get()
	require.GreaterOrEqual(t, buf2.Cap(), SmallBufferSize)
	pool2.Put(buf2)
}

func TestBufferPool_LargeBufferNotReturned(t *testing.T) {
	pool := NewBufferPool(1024)

	buf := pool.Get()
	// Grow buffer significantly beyond pool size
	largeData := make([]byte, 1024*1024)
	buf.Write(largeData)

	// Put it back - should not actually go to pool
	pool.Put(buf)

	// Get a new buffer - should be fresh, not the large one
	buf2 := pool.Get()
	require.LessOrEqual(t, buf2.Cap(), 1024*4) // Allow some growth
}

func TestByteSlicePool_Basic(t *testing.T) {
	pool := NewByteSlicePool(1024)

	// Get a slice
	b := pool.Get()
	require.NotNil(t, b)
	require.Equal(t, 1024, len(b))

	// Modify it
	b[0] = 42

	// Put it back
	pool.Put(b)

	// Get with specific size
	b2 := pool.GetWithSize(512)
	require.Equal(t, 512, len(b2))

	// Get with size larger than pool
	b3 := pool.GetWithSize(2048)
	require.Equal(t, 2048, len(b3))
}

func TestByteSlicePool_NilPut(t *testing.T) {
	pool := NewByteSlicePool(1024)
	pool.Put(nil) // Should not panic
}

func TestByteSlicePool_WrongSizeNotReturned(t *testing.T) {
	pool := NewByteSlicePool(1024)

	// Slice with different capacity should not be returned to pool
	wrongSize := make([]byte, 512)
	pool.Put(wrongSize) // Should just discard

	// Get should give correct size
	b := pool.Get()
	require.Equal(t, 1024, cap(b))
}

func TestGlobalPools(t *testing.T) {
	// Test SmallBufferPool
	buf := SmallBufferPool.Get()
	require.NotNil(t, buf)
	SmallBufferPool.Put(buf)

	// Test MediumBufferPool
	buf = MediumBufferPool.Get()
	require.NotNil(t, buf)
	MediumBufferPool.Put(buf)

	// Test LargeBufferPool
	buf = LargeBufferPool.Get()
	require.NotNil(t, buf)
	LargeBufferPool.Put(buf)

	// Test SmallBytePool
	b := SmallBytePool.Get()
	require.NotNil(t, b)
	SmallBytePool.Put(b)

	// Test MediumBytePool
	b = MediumBytePool.Get()
	require.NotNil(t, b)
	MediumBytePool.Put(b)
}

func TestGetBuffer(t *testing.T) {
	// Small hint
	buf := GetBuffer(1000)
	require.NotNil(t, buf)
	PutBuffer(buf)

	// Medium hint
	buf = GetBuffer(10000)
	require.NotNil(t, buf)
	PutBuffer(buf)

	// Large hint
	buf = GetBuffer(500000)
	require.NotNil(t, buf)
	PutBuffer(buf)
}

func TestPutBuffer_Nil(t *testing.T) {
	PutBuffer(nil) // Should not panic
}

func TestArena_Basic(t *testing.T) {
	arena := NewArena(1024)

	require.Equal(t, 1024, arena.Capacity())
	require.Equal(t, 1024, arena.Available())
	require.Equal(t, 0, arena.Used())

	// Allocate some bytes
	b1 := arena.Alloc(100)
	require.NotNil(t, b1)
	require.Equal(t, 100, len(b1))
	require.Equal(t, 100, arena.Used())
	require.Equal(t, 924, arena.Available())

	// Allocate more
	b2 := arena.Alloc(200)
	require.NotNil(t, b2)
	require.Equal(t, 200, len(b2))
	require.Equal(t, 300, arena.Used())

	// Allocations should not overlap
	b1[0] = 1
	b2[0] = 2
	require.Equal(t, byte(1), b1[0])
	require.Equal(t, byte(2), b2[0])
}

func TestArena_Reset(t *testing.T) {
	arena := NewArena(1024)

	arena.Alloc(500)
	require.Equal(t, 500, arena.Used())

	arena.Reset()
	require.Equal(t, 0, arena.Used())
	require.Equal(t, 1024, arena.Available())

	// Can allocate again
	b := arena.Alloc(1024)
	require.NotNil(t, b)
	require.Equal(t, 1024, arena.Used())
}

func TestArena_Overflow(t *testing.T) {
	arena := NewArena(100)

	// Allocate most of the space
	b1 := arena.Alloc(80)
	require.NotNil(t, b1)

	// Try to allocate more than available
	b2 := arena.Alloc(50)
	require.Nil(t, b2)

	// Can still allocate what's left
	b3 := arena.Alloc(20)
	require.NotNil(t, b3)
}

func TestArena_DefaultCapacity(t *testing.T) {
	arena := NewArena(0)
	require.Equal(t, LargeBufferSize, arena.Capacity())

	arena2 := NewArena(-100)
	require.Equal(t, LargeBufferSize, arena2.Capacity())
}

func TestArena_Concurrent(t *testing.T) {
	arena := NewArena(10000)
	var wg sync.WaitGroup

	// Multiple goroutines allocating concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := arena.Alloc(10)
			if b != nil {
				// Write to verify no overlap
				for j := range b {
					b[j] = byte(j)
				}
			}
		}()
	}

	wg.Wait()
	require.LessOrEqual(t, arena.Used(), 1000) // 100 * 10
}

func TestBufferPool_Concurrent(t *testing.T) {
	pool := NewBufferPool(1024)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			buf.WriteString("test data")
			pool.Put(buf)
		}()
	}

	wg.Wait()
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.WriteString("benchmark test data")
		pool.Put(buf)
	}
}

func BenchmarkBufferAlloc_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, 4096))
		buf.WriteString("benchmark test data")
		_ = buf
	}
}

func BenchmarkByteSlicePool_GetPut(b *testing.B) {
	pool := NewByteSlicePool(4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf[0] = 42
		pool.Put(buf)
	}
}

func BenchmarkByteSliceAlloc_NoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 4096)
		buf[0] = 42
		_ = buf
	}
}

func BenchmarkArena_Alloc(b *testing.B) {
	arena := NewArena(b.N * 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arena.Alloc(100)
	}
}

func BenchmarkArena_AllocWithReset(b *testing.B) {
	arena := NewArena(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if arena.Available() < 100 {
			arena.Reset()
		}
		arena.Alloc(100)
	}
}
