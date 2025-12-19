package log

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
)

func setupTestLogger() (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = ""
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	)
	zapLogger := zap.New(core)
	logger := Logger{Logger: zapr.NewLogger(zapLogger)}
	return &logger, &buf
}

func TestLogger_Error(t *testing.T) {
	logger, buf := setupTestLogger()
	testErr := errors.New("test error")

	logger.Error(testErr, "error message", "key", "value")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "error message")
	assertContains(t, output, "test error")
	assertContainsKeyValue(t, output, "key", "value")
}

func TestLogger_Warning(t *testing.T) {
	logger, buf := setupTestLogger()

	logger.Warning("warning message", "key", "value")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "warning message")
	assertContainsKeyValue(t, output, "key", "value")
}

func TestLogger_Info(t *testing.T) {
	logger, buf := setupTestLogger()

	logger.Info("info message", "key", "value")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "info message")
	assertContainsKeyValue(t, output, "key", "value")
}

func TestLogger_Debug(t *testing.T) {
	logger, buf := setupTestLogger()

	logger.Debug("debug message", "key", "value")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "\"level\":\"debug\"") // Zap adds level info
	assertContains(t, output, "debug message")
	assertContainsKeyValue(t, output, "key", "value")
}

func TestGetLogger(t *testing.T) {
	// Create a test logger and add it to context
	var buf bytes.Buffer
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = ""
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	zapLogger := zap.New(core)
	testLogger := zapr.NewLogger(zapLogger)

	ctx := ctrl.LoggerInto(context.Background(), testLogger)

	// Get the logger from context
	logger := GetLogger(ctx)

	// Test that it works
	logger.Info("test message")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "test message")
}

func TestLogger_MultipleKeyValuePairs(t *testing.T) {
	logger, buf := setupTestLogger()

	logger.Info("test message", "key1", "value1", "key2", 42, "key3", true)

	output := buf.String()
	assertContainsKeyValue(t, output, "key1", "value1")
	assertContainsKeyValue(t, output, "key2", "42")
	assertContainsKeyValue(t, output, "key3", "true")
}

func TestLogger_EmptyKeyValuePairs(t *testing.T) {
	logger, buf := setupTestLogger()

	logger.Info("test message")

	output := buf.String()
	assertNotEmpty(t, output)
	assertContains(t, output, "test message")
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func assertNotEmpty(t *testing.T, output string) {
	t.Helper()
	if output == "" {
		t.Fatal("Expected log output, got empty string")
	}
}

func assertContains(t *testing.T, output, expected string) {
	t.Helper()
	if !contains(output, expected) {
		t.Errorf("Expected log to contain '%s', got: %s", expected, output)
	}
}

func assertContainsKeyValue(t *testing.T, output, key, value string) {
	t.Helper()
	if !contains(output, key) || !contains(output, value) {
		t.Errorf("Expected log to contain key-value pair %s/%s, got: %s", key, value, output)
	}
}
