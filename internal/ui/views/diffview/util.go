package diffview

import (
	"context"
	"strconv"
	"time"
)

// callTimeout bounds one backend call.
const callTimeout = 30 * time.Second

// ctxTimeout returns the context of one backend call.
func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), callTimeout)
}

func itoa(n int) string { return strconv.Itoa(n) }
