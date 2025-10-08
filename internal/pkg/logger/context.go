package logger

import "context"

type contextKey string

const (
	// RequestIDKey is the key for request ID in context
	RequestIDKey contextKey = "request_id"
	// CorrelationIDKey is the key for correlation ID in context
	CorrelationIDKey contextKey = "correlation_id"
	// UserIDKey is the key for user ID in context
	UserIDKey contextKey = "user_id"
)

// WithRequestID adds request ID to context and logger
func WithRequestID(ctx context.Context, requestID string) context.Context {
	ctx = context.WithValue(ctx, RequestIDKey, requestID)

	// Add to logger in context
	logger := FromContext(ctx)
	zl := logger.With().Str("request_id", requestID).Logger()
	return zl.WithContext(ctx)
}

// WithCorrelationID adds correlation ID to context and logger
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)

	logger := FromContext(ctx)
	zl := logger.With().Str("correlation_id", correlationID).Logger()
	return zl.WithContext(ctx)
}

// WithUserID adds user ID to context and logger
func WithUserID(ctx context.Context, userID string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)

	logger := FromContext(ctx)
	zl := logger.With().Str("user_id", userID).Logger()
	return zl.WithContext(ctx)
}

// WithFields adds multiple fields to context logger
func WithFields(ctx context.Context, fields map[string]interface{}) context.Context {
	logger := FromContext(ctx)
	zl := logger.With().Fields(fields).Logger()
	return zl.WithContext(ctx)
}

// GetRequestID gets request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetCorrelationID gets correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserID gets user ID from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}
