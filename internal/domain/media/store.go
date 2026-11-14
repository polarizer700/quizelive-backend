package media

import (
	"context"
	"io"
)

// Store — файлы вопросов. Сейчас диск в РФ, позже можно подставить российский S3.
type Store interface {
	Put(ctx context.Context, key string, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	URL(key string) string
}
