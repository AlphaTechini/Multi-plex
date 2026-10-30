package transport

import "errors"

// ErrAddrInUse is returned by Listen when a running daemon already answers at
// the address. Callers should attach to that daemon instead of starting one.
var ErrAddrInUse = errors.New("transport: address in use by a running daemon")
