package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	// DEBUG level for detailed troubleshooting information
	DEBUG LogLevel = iota
	// INFO level for general operational information
	INFO
	// WARN level for potentially harmful situations
	WARN
	// ERROR level for error events that might still allow the application to continue
	ERROR
	// FATAL level for severe error events that will likely lead the application to abort
	FATAL
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides logging functionality with different severity levels
type Logger struct {
	level  LogLevel
	writer io.Writer
	logger *log.Logger
}

// LoggerOption defines a functional option for configuring the logger
type LoggerOption func(*Logger)

// WithLevel sets the minimum log level to be displayed
func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

// WithWriter sets the output writer for the logger
func WithWriter(writer io.Writer) LoggerOption {
	return func(l *Logger) {
		l.writer = writer
		l.logger = log.New(writer, "", 0)
	}
}

// WithFile sets a file as the output for the logger
func WithFile(filename string) LoggerOption {
	return func(l *Logger) {
		// Create directory if it doesn't exist
		dir := filepath.Dir(filename)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Failed to create log directory: %v", err)
				return
			}
		}

		// Open or create the log file
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
			return
		}

		// Use both stdout and the file as writers
		multiWriter := io.MultiWriter(os.Stdout, file)
		l.writer = multiWriter
		l.logger = log.New(multiWriter, "", 0)
	}
}

// NewLogger creates a new logger with the given options
func NewLogger(options ...LoggerOption) *Logger {
	// Default to INFO level and stdout
	logger := &Logger{
		level:  INFO,
		writer: os.Stdout,
		logger: log.New(os.Stdout, "", 0),
	}

	// Apply options
	for _, option := range options {
		option(logger)
	}

	return logger
}

// formatMessage formats a log message with timestamp, level, file info, and key-value pairs
func (l *Logger) formatMessage(level LogLevel, message string, keyValues ...interface{}) string {
	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	fileInfo := "???"
	if ok {
		fileInfo = filepath.Base(file)
	}

	// Format timestamp
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	// Format key-value pairs
	kvPairs := ""
	if len(keyValues) > 0 {
		pairs := make([]string, 0, len(keyValues)/2)
		for i := 0; i < len(keyValues); i += 2 {
			if i+1 < len(keyValues) {
				key := fmt.Sprintf("%v", keyValues[i])
				value := fmt.Sprintf("%v", keyValues[i+1])
				pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
			}
		}
		if len(pairs) > 0 {
			kvPairs = " " + strings.Join(pairs, " ")
		}
	}

	return fmt.Sprintf("%s [%s] %s:%d: %s%s", timestamp, level.String(), fileInfo, line, message, kvPairs)
}

// log logs a message with the given level
func (l *Logger) log(level LogLevel, message string, keyValues ...interface{}) {
	if level >= l.level {
		l.logger.Println(l.formatMessage(level, message, keyValues...))
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, keyValues ...interface{}) {
	l.log(DEBUG, message, keyValues...)
}

// Info logs an informational message
func (l *Logger) Info(message string, keyValues ...interface{}) {
	l.log(INFO, message, keyValues...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, keyValues ...interface{}) {
	l.log(WARN, message, keyValues...)
}

// Error logs an error message
func (l *Logger) Error(message string, keyValues ...interface{}) {
	l.log(ERROR, message, keyValues...)
}

// Fatal logs a fatal message and exits the application
func (l *Logger) Fatal(message string, keyValues ...interface{}) {
	l.log(FATAL, message, keyValues...)
	os.Exit(1)
}

// WithContext returns a new logger with additional context values
func (l *Logger) WithContext(keyValues ...interface{}) *Logger {
	// Create a new logger with the same configuration
	newLogger := &Logger{
		level:  l.level,
		writer: l.writer,
		logger: l.logger,
	}

	// Add custom log wrapper that includes the context values
	originalLog := newLogger.log
	newLogger.log = func(level LogLevel, message string, msgKeyValues ...interface{}) {
		// Combine context values with message-specific values
		allKeyValues := append(keyValues, msgKeyValues...)
		originalLog(level, message, allKeyValues...)
	}

	return newLogger
}
