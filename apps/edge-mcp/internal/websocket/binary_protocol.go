package websocket

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
)

// Binary protocol constants
const (
	BinaryProtocolVersion = 1
	HeaderSize            = 12 // version(1) + flags(1) + messageType(2) + payloadSize(4) + reserved(4)
)

// Binary flags
const (
	FlagCompressed = 1 << 0
	FlagEncrypted  = 1 << 1
)

// BinaryEncoder handles binary message encoding
type BinaryEncoder struct {
	compressionThreshold int
}

// NewBinaryEncoder creates a new binary encoder
func NewBinaryEncoder(compressionThreshold int) *BinaryEncoder {
	return &BinaryEncoder{
		compressionThreshold: compressionThreshold,
	}
}

// Encode encodes a message to binary format
func (be *BinaryEncoder) Encode(msg *ws.Message) ([]byte, error) {
	// Marshal message to JSON
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Check if compression is needed
	var flags byte
	if len(payload) > be.compressionThreshold {
		compressed, err := compressPayload(payload)
		if err == nil && len(compressed) < len(payload) {
			payload = compressed
			flags |= FlagCompressed
		}
	}

	// Create header
	header := make([]byte, HeaderSize)
	header[0] = BinaryProtocolVersion
	header[1] = flags
	binary.BigEndian.PutUint16(header[2:4], uint16(msg.Type))

	// Ensure payload length fits in uint32
	payloadLen := len(payload)
	if payloadLen < 0 || payloadLen > int(^uint32(0)) {
		return nil, fmt.Errorf("payload length %d exceeds uint32 range", payloadLen)
	}
	binary.BigEndian.PutUint32(header[4:8], uint32(payloadLen))
	// header[8:12] reserved for future use

	// Combine header and payload
	result := append(header, payload...)

	return result, nil
}

// Decode decodes a binary message
func (be *BinaryEncoder) Decode(data []byte) (*ws.Message, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	// Parse header
	version := data[0]
	if version != BinaryProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", version)
	}

	flags := data[1]
	messageType := binary.BigEndian.Uint16(data[2:4])
	payloadSize := binary.BigEndian.Uint32(data[4:8])

	// Validate payload size
	if len(data) < HeaderSize+int(payloadSize) {
		return nil, fmt.Errorf("incomplete message: expected %d bytes, got %d", HeaderSize+payloadSize, len(data))
	}

	// Extract payload
	payload := data[HeaderSize : HeaderSize+payloadSize]

	// Decompress if needed
	if flags&FlagCompressed != 0 {
		decompressed, err := decompressPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress payload: %w", err)
		}
		payload = decompressed
	}

	// Unmarshal message
	msg := GetMessage()
	if err := json.Unmarshal(payload, msg); err != nil {
		PutMessage(msg)
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Override message type from header
	// Ensure messageType fits in uint8 (ws.MessageType)
	if messageType > 255 {
		PutMessage(msg)
		return nil, fmt.Errorf("invalid message type: %d exceeds uint8 range", messageType)
	}
	msg.Type = ws.MessageType(messageType)

	return msg, nil
}

// Compression helpers

func compressPayload(data []byte) ([]byte, error) {
	buf := GetBuffer()
	defer PutBuffer(buf)

	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompressPayload(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail decompression
			_ = err
		}
	}()

	buf := GetBuffer()
	defer PutBuffer(buf)

	// Limit decompressed size to prevent decompression bombs
	const maxDecompressedSize = 10 * 1024 * 1024 // 10MB
	limitedReader := io.LimitReader(reader, maxDecompressedSize)

	n, err := io.Copy(buf, limitedReader)
	if err != nil {
		return nil, err
	}

	// Check if we hit the limit
	if n == maxDecompressedSize {
		// Try to read one more byte to see if there's more data
		if _, err := reader.Read(make([]byte, 1)); err != io.EOF {
			return nil, fmt.Errorf("decompressed data exceeds maximum size of %d bytes", maxDecompressedSize)
		}
	}

	return buf.Bytes(), nil
}

// MessageBatcher batches multiple messages for efficient transmission
type MessageBatcher struct {
	messages      []*ws.Message
	maxBatchSize  int
	maxBatchDelay time.Duration
	flushChan     chan struct{}
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(maxSize int, maxDelay time.Duration) *MessageBatcher {
	return &MessageBatcher{
		messages:      make([]*ws.Message, 0, maxSize),
		maxBatchSize:  maxSize,
		maxBatchDelay: maxDelay,
		flushChan:     make(chan struct{}, 1),
	}
}

// Add adds a message to the batch
func (mb *MessageBatcher) Add(msg *ws.Message) {
	mb.messages = append(mb.messages, msg)

	if len(mb.messages) >= mb.maxBatchSize {
		select {
		case mb.flushChan <- struct{}{}:
		default:
		}
	}
}

// Flush returns all batched messages
func (mb *MessageBatcher) Flush() []*ws.Message {
	if len(mb.messages) == 0 {
		return nil
	}

	batch := mb.messages
	mb.messages = make([]*ws.Message, 0, mb.maxBatchSize)
	return batch
}
