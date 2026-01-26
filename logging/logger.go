package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Logger is a structured logger interface for blockberry.
// It wraps slog.Logger with convenience methods for common logging patterns.
type Logger struct {
	*slog.Logger
}

// New creates a new Logger with the given handler.
func New(handler slog.Handler) *Logger {
	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewTextLogger creates a new Logger with text output format.
func NewTextLogger(w io.Writer, level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}
	return New(slog.NewTextHandler(w, opts))
}

// NewJSONLogger creates a new Logger with JSON output format.
func NewJSONLogger(w io.Writer, level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}
	return New(slog.NewJSONHandler(w, opts))
}

// NewDevelopmentLogger creates a logger suitable for development.
// Uses text format with debug level output to stderr.
func NewDevelopmentLogger() *Logger {
	return NewTextLogger(os.Stderr, slog.LevelDebug)
}

// NewProductionLogger creates a logger suitable for production.
// Uses JSON format with info level output to stdout.
func NewProductionLogger() *Logger {
	return NewJSONLogger(os.Stdout, slog.LevelInfo)
}

// NewNopLogger creates a logger that discards all output.
func NewNopLogger() *Logger {
	return New(nopHandler{})
}

// With returns a new Logger with the given attributes added to every log entry.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// WithComponent returns a new Logger with a component attribute.
func (l *Logger) WithComponent(name string) *Logger {
	return l.With(Component(name))
}

// WithPeer returns a new Logger with a peer attribute.
func (l *Logger) WithPeer(id peer.ID) *Logger {
	return l.With(PeerID(id))
}

// WithStream returns a new Logger with a stream attribute.
func (l *Logger) WithStream(name string) *Logger {
	return l.With(Stream(name))
}

// Common attribute constructors for blockchain-specific fields.

// Component creates a component attribute for identifying the source module.
func Component(name string) slog.Attr {
	return slog.String("component", name)
}

// PeerID creates a peer ID attribute.
func PeerID(id peer.ID) slog.Attr {
	return slog.String("peer_id", id.String())
}

// PeerIDStr creates a peer ID attribute from a string.
func PeerIDStr(id string) slog.Attr {
	return slog.String("peer_id", id)
}

// Height creates a block height attribute.
func Height(h int64) slog.Attr {
	return slog.Int64("height", h)
}

// Hash creates a hash attribute (hex-encoded).
func Hash(h []byte) slog.Attr {
	return slog.String("hash", bytesToHex(h))
}

// TxHash creates a transaction hash attribute (hex-encoded).
func TxHash(h []byte) slog.Attr {
	return slog.String("tx_hash", bytesToHex(h))
}

// BlockHash creates a block hash attribute (hex-encoded).
func BlockHash(h []byte) slog.Attr {
	return slog.String("block_hash", bytesToHex(h))
}

// Stream creates a stream name attribute.
func Stream(name string) slog.Attr {
	return slog.String("stream", name)
}

// MsgType creates a message type attribute.
func MsgType(t string) slog.Attr {
	return slog.String("msg_type", t)
}

// Duration creates a duration attribute in milliseconds.
func Duration(d time.Duration) slog.Attr {
	return slog.Float64("duration_ms", float64(d.Nanoseconds())/1e6)
}

// DurationSeconds creates a duration attribute in seconds.
func DurationSeconds(d time.Duration) slog.Attr {
	return slog.Float64("duration_s", d.Seconds())
}

// Latency creates a latency attribute in milliseconds.
func Latency(d time.Duration) slog.Attr {
	return slog.Float64("latency_ms", float64(d.Nanoseconds())/1e6)
}

// Count creates a count attribute.
func Count(n int) slog.Attr {
	return slog.Int("count", n)
}

// Size creates a size attribute in bytes.
func Size(n int) slog.Attr {
	return slog.Int("size_bytes", n)
}

// Version creates a version attribute.
func Version(v int64) slog.Attr {
	return slog.Int64("version", v)
}

// ChainID creates a chain ID attribute.
func ChainID(id string) slog.Attr {
	return slog.String("chain_id", id)
}

// NodeID creates a node ID attribute.
func NodeID(id string) slog.Attr {
	return slog.String("node_id", id)
}

// Address creates an address attribute.
func Address(addr string) slog.Attr {
	return slog.String("address", addr)
}

// Direction creates a connection direction attribute.
func Direction(isOutbound bool) slog.Attr {
	dir := "inbound"
	if isOutbound {
		dir = "outbound"
	}
	return slog.String("direction", dir)
}

// Error creates an error attribute.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("error", err.Error())
}

// Reason creates a reason attribute.
func Reason(r string) slog.Attr {
	return slog.String("reason", r)
}

// State creates a state attribute.
func State(s string) slog.Attr {
	return slog.String("state", s)
}

// Progress creates a progress attribute (0.0 to 1.0).
func Progress(p float64) slog.Attr {
	return slog.Float64("progress", p)
}

// BatchSize creates a batch size attribute.
func BatchSize(n int) slog.Attr {
	return slog.Int("batch_size", n)
}

// Index creates an index attribute.
func Index(n int) slog.Attr {
	return slog.Int("index", n)
}

// bytesToHex converts bytes to hex string.
func bytesToHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	const hexDigits = "0123456789abcdef"
	hex := make([]byte, len(b)*2)
	for i, v := range b {
		hex[i*2] = hexDigits[v>>4]
		hex[i*2+1] = hexDigits[v&0x0f]
	}
	return string(hex)
}

// nopHandler is a slog.Handler that discards all logs.
type nopHandler struct{}

func (nopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (nopHandler) Handle(context.Context, slog.Record) error { return nil }
func (h nopHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h nopHandler) WithGroup(string) slog.Handler           { return h }
