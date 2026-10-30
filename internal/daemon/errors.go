package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

func badRequest(format string, a ...any) *protocol.Error {
	return &protocol.Error{Code: protocol.CodeBadRequest, Message: fmt.Sprintf(format, a...)}
}

func notFound(format string, a ...any) *protocol.Error {
	return &protocol.Error{Code: protocol.CodeNotFound, Message: fmt.Sprintf(format, a...)}
}

func denied(format string, a ...any) *protocol.Error {
	return &protocol.Error{Code: protocol.CodeDenied, Message: fmt.Sprintf(format, a...)}
}

// toProtocolError maps any error to the wire error for a response.
func toProtocolError(err error) *protocol.Error {
	var pe *protocol.Error
	switch {
	case errors.As(err, &pe):
		return pe
	case errors.Is(err, store.ErrNotFound):
		return &protocol.Error{Code: protocol.CodeNotFound, Message: err.Error()}
	case errors.Is(err, context.Canceled):
		return &protocol.Error{Code: protocol.CodeInternal, Message: "request cancelled"}
	}
	return &protocol.Error{Code: protocol.CodeInternal, Message: err.Error()}
}
