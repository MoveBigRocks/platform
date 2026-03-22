package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zap.SugaredLogger for structured logging
type Logger struct {
	*zap.SugaredLogger
}

// New creates a new logger instance with production-ready defaults
func New() *Logger {
	return NewWithEnvironment(os.Getenv("ENVIRONMENT"))
}

// NewWithEnvironment creates a logger with specific environment settings
func NewWithEnvironment(env string) *Logger {
	var config zap.Config

	if env == "production" {
		config = zap.NewProductionConfig()
		// Production: JSON format, info level
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

		// Always log to stdout for container logs
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}

		// Optionally log to file with rotation if LOG_FILE_PATH is set
		if logFilePath := os.Getenv("LOG_FILE_PATH"); logFilePath != "" {
			// Create log directory if it doesn't exist
			logDir := filepath.Dir(logFilePath)
			if err := os.MkdirAll(logDir, 0755); err == nil {
				fileWriter := zapcore.AddSync(&lumberjack.Logger{
					Filename:   logFilePath,
					MaxSize:    100, // megabytes
					MaxBackups: 10,  // keep 10 old log files
					MaxAge:     30,  // days
					Compress:   true,
				})

				// Create file core with JSON encoder
				fileCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(config.EncoderConfig),
					fileWriter,
					config.Level,
				)

				// Create stdout core
				stdoutCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(config.EncoderConfig),
					zapcore.Lock(os.Stdout),
					config.Level,
				)

				// Combine both cores
				core := zapcore.NewTee(fileCore, stdoutCore)
				logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
				return &Logger{logger.Sugar()}
			}
		}
	} else {
		config = zap.NewDevelopmentConfig()
		// Development: Console format, debug level
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	}

	// Standardize timestamp field
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	return &Logger{logger.Sugar()}
}

// WithField adds a single field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.SugaredLogger.With(key, value)}
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{l.SugaredLogger.With(args...)}
}

// WithError adds an error field to the logger context
func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

// WithWorkspace adds workspace context
func (l *Logger) WithWorkspace(workspaceID string) *Logger {
	return l.WithField("workspace_id", workspaceID)
}

// WithUser adds user context
func (l *Logger) WithUser(userID string) *Logger {
	return l.WithField("user_id", userID)
}

// WithProject adds project context (for error monitoring)
func (l *Logger) WithProject(projectID string) *Logger {
	return l.WithField("project_id", projectID)
}

// WithRequestID adds request tracing context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// WithCorrelationID adds correlation ID for distributed event tracing
func (l *Logger) WithCorrelationID(correlationID string) *Logger {
	return l.WithField("correlation_id", correlationID)
}

// WithEventContext adds event context from eventbus correlation
func (l *Logger) WithEventContext(correlationID, parentEventID string) *Logger {
	logger := l
	if correlationID != "" {
		logger = logger.WithField("correlation_id", correlationID)
	}
	if parentEventID != "" {
		logger = logger.WithField("parent_event_id", parentEventID)
	}
	return logger
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}

// NewNop returns a no-op Logger that discards all log output.
// Useful for testing or when logging should be disabled.
func NewNop() *Logger {
	return &Logger{zap.NewNop().Sugar()}
}
