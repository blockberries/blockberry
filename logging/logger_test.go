package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTextLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)
	require.NotNil(t, logger)

	logger.Info("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestNewJSONLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJSONLogger(buf, slog.LevelInfo)
	require.NotNil(t, logger)

	logger.Info("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, `"msg":"test message"`)
	assert.Contains(t, output, `"key":"value"`)

	// Verify it's valid JSON
	var parsed map[string]any
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "test message", parsed["msg"])
	assert.Equal(t, "value", parsed["key"])
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger := NewDevelopmentLogger()
	require.NotNil(t, logger)
	// Just verify it can log without panicking
	logger.Debug("debug message")
	logger.Info("info message")
}

func TestNewProductionLogger(t *testing.T) {
	logger := NewProductionLogger()
	require.NotNil(t, logger)
	// Just verify it can log without panicking
	logger.Info("info message")
}

func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	require.NotNil(t, logger)

	// NopLogger should not panic and should discard all output
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestLogger_With(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	childLogger := logger.With("parent_key", "parent_value")
	require.NotNil(t, childLogger)

	childLogger.Info("child message", "child_key", "child_value")

	output := buf.String()
	assert.Contains(t, output, "parent_key=parent_value")
	assert.Contains(t, output, "child_key=child_value")
}

func TestLogger_WithComponent(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	compLogger := logger.WithComponent("pex")
	compLogger.Info("component message")

	output := buf.String()
	assert.Contains(t, output, "component=pex")
}

func TestLogger_WithPeer(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	peerID := peer.ID("12D3KooWTest")
	peerLogger := logger.WithPeer(peerID)
	peerLogger.Info("peer message")

	output := buf.String()
	// peer.ID.String() uses base58 encoding, so just check the attribute name exists
	assert.Contains(t, output, "peer_id=")
}

func TestLogger_WithStream(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	streamLogger := logger.WithStream("transactions")
	streamLogger.Info("stream message")

	output := buf.String()
	assert.Contains(t, output, "stream=transactions")
}

func TestAttributeConstructors(t *testing.T) {
	tests := []struct {
		name     string
		attr     slog.Attr
		expected string
	}{
		{"Component", Component("sync"), "component=sync"},
		{"PeerIDStr", PeerIDStr("peer123"), "peer_id=peer123"},
		{"Height", Height(12345), "height=12345"},
		{"Hash", Hash([]byte{0xde, 0xad, 0xbe, 0xef}), "hash=deadbeef"},
		{"TxHash", TxHash([]byte{0xca, 0xfe}), "tx_hash=cafe"},
		{"BlockHash", BlockHash([]byte{0xba, 0xbe}), "block_hash=babe"},
		{"Stream", Stream("consensus"), "stream=consensus"},
		{"MsgType", MsgType("HelloRequest"), "msg_type=HelloRequest"},
		{"Count", Count(42), "count=42"},
		{"Size", Size(1024), "size_bytes=1024"},
		{"Version", Version(5), "version=5"},
		{"ChainID", ChainID("testnet-1"), "chain_id=testnet-1"},
		{"NodeID", NodeID("node123"), "node_id=node123"},
		{"Address", Address("/ip4/127.0.0.1/tcp/26656"), "address=/ip4/127.0.0.1/tcp/26656"},
		{"DirectionInbound", Direction(false), "direction=inbound"},
		{"DirectionOutbound", Direction(true), "direction=outbound"},
		{"Reason", Reason("timeout"), "reason=timeout"},
		{"State", State("syncing"), "state=syncing"},
		{"BatchSize", BatchSize(100), "batch_size=100"},
		{"Index", Index(5), "index=5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := NewTextLogger(buf, slog.LevelInfo)
			logger.Info("test", tt.attr)

			output := buf.String()
			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestDurationAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJSONLogger(buf, slog.LevelInfo)

	d := 150 * time.Millisecond
	logger.Info("test", Duration(d))

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.InDelta(t, 150.0, parsed["duration_ms"], 0.1)
}

func TestDurationSecondsAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJSONLogger(buf, slog.LevelInfo)

	d := 2500 * time.Millisecond
	logger.Info("test", DurationSeconds(d))

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.InDelta(t, 2.5, parsed["duration_s"], 0.01)
}

func TestLatencyAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJSONLogger(buf, slog.LevelInfo)

	d := 50 * time.Millisecond
	logger.Info("test", Latency(d))

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.InDelta(t, 50.0, parsed["latency_ms"], 0.1)
}

func TestProgressAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJSONLogger(buf, slog.LevelInfo)

	logger.Info("test", Progress(0.75))

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.InDelta(t, 0.75, parsed["progress"], 0.001)
}

func TestErrorAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	err := assert.AnError
	logger.Info("test", Error(err))

	output := buf.String()
	assert.Contains(t, output, "error=")
}

func TestErrorAttribute_Nil(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	// Nil error should produce empty attribute
	logger.Info("test", Error(nil))

	output := buf.String()
	// Should not contain "error=" when error is nil
	assert.NotContains(t, output, "error=")
}

func TestHashAttribute_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	logger.Info("test", Hash(nil))

	output := buf.String()
	assert.Contains(t, output, `hash=""`)
}

func TestLogLevels(t *testing.T) {
	// Test that log levels filter correctly
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelWarn)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestBytesToHex(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{nil, ""},
		{[]byte{}, ""},
		{[]byte{0x00}, "00"},
		{[]byte{0xff}, "ff"},
		{[]byte{0xde, 0xad, 0xbe, 0xef}, "deadbeef"},
		{[]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}, "0123456789abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := bytesToHex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNopHandler(t *testing.T) {
	h := nopHandler{}

	// All methods should be no-ops
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	assert.False(t, h.Enabled(context.Background(), slog.LevelError))
	assert.NoError(t, h.Handle(context.Background(), slog.Record{}))
	assert.Equal(t, h, h.WithAttrs(nil))
	assert.Equal(t, h, h.WithGroup("test"))
}

func TestChainedWith(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	chainedLogger := logger.
		WithComponent("sync").
		WithStream("blocksync").
		With("custom", "value")

	chainedLogger.Info("chained message")

	output := buf.String()
	assert.Contains(t, output, "component=sync")
	assert.Contains(t, output, "stream=blocksync")
	assert.Contains(t, output, "custom=value")
	assert.Contains(t, output, "chained message")
}

func TestPeerIDAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTextLogger(buf, slog.LevelInfo)

	// Test with a real peer.ID
	peerID := peer.ID("QmTest12345")
	logger.Info("test", PeerID(peerID))

	output := buf.String()
	assert.True(t, strings.Contains(output, "peer_id="))
}
