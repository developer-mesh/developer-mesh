package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/mock"

	ws "github.com/developer-mesh/developer-mesh/pkg/models/websocket"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MockLogger implements observability.Logger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) WithFields(fields map[string]interface{}) observability.Logger {
	args := m.Called(fields)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) WithError(err error) observability.Logger {
	args := m.Called(err)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) With(fields map[string]interface{}) observability.Logger {
	args := m.Called(fields)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	args := m.Called(prefix)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(observability.Logger)
}

// NewTestLogger creates a mock logger with default expectations
func NewTestLogger() *MockLogger {
	logger := &MockLogger{}
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Info", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	logger.On("Error", mock.Anything, mock.Anything).Return()
	logger.On("Fatal", mock.Anything, mock.Anything).Return()
	logger.On("Debugf", mock.Anything, mock.Anything).Return()
	logger.On("Infof", mock.Anything, mock.Anything).Return()
	logger.On("Warnf", mock.Anything, mock.Anything).Return()
	logger.On("Errorf", mock.Anything, mock.Anything).Return()
	logger.On("Fatalf", mock.Anything, mock.Anything).Return()
	logger.On("WithFields", mock.Anything).Return(logger)
	logger.On("WithError", mock.Anything).Return(logger)
	logger.On("With", mock.Anything).Return(logger)
	logger.On("WithPrefix", mock.Anything).Return(logger)
	return logger
}

// NewConnection creates a new connection for testing
func NewConnection(id string, conn *websocket.Conn, hub *Server) *Connection {
	wsConn := &ws.Connection{
		ID:        id,
		AgentID:   "",
		TenantID:  "",
		CreatedAt: time.Now(),
		LastPing:  time.Now(),
	}
	wsConn.SetState(ws.ConnectionStateConnecting)

	c := &Connection{
		Connection: wsConn,
		conn:       conn,
		hub:        hub,
		send:       make(chan []byte, 256),
		afterSend:  make(chan *PostActionConfig, 32), // Buffered to prevent blocking
		closed:     make(chan struct{}),
		closeOnce:  sync.Once{},
		wg:         sync.WaitGroup{},
	}

	return c
}

// testContext creates a context for testing
func testContext() context.Context {
	return context.Background()
}
