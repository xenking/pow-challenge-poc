package server

import (
	"context"
	"fmt"
	"net"
	"time"
)

// RequestContext contains incoming request and manages outgoing response.
//
// It is forbidden copying RequestContext instances.
//
// RequestHandler should avoid holding references to incoming RequestContext and/or
// its members after the return.
// If holding RequestContext references after the return is unavoidable
// (for instance, ctx is passed to a separate goroutine and ctx lifetime cannot
// be controlled), then the RequestHandler MUST call ctx.TimeoutError()
// before return.
//
// It is unsafe modifying/reading RequestContext instance from concurrently
// running goroutines. The only exception is TimeoutError*, which may be called
// while other goroutines accessing RequestContext.
type RequestContext struct {
	noCopy noCopy

	// Incoming request.
	//
	// Copying Request by value is forbidden. Use pointer to Request instead.
	Request Request

	// Outgoing response.
	//
	// Copying Response by value is forbidden. Use pointer to Response instead.
	Response Response

	// request-bound context inherited from server context.
	ctx    context.Context
	cancel context.CancelFunc

	connID     uint64
	remoteAddr net.Addr

	s   *Server
	c   net.Conn
	fbr firstByteReader

	timeoutResponse *Response
	timeoutCh       chan struct{}
	timeoutTimer    *time.Timer
}

func (ctx *RequestContext) reset() {
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.fbr.reset()

	ctx.connID = 0
	ctx.remoteAddr = nil
	ctx.c = nil

	ctx.ctx = nil
	if ctx.cancel != nil {
		ctx.cancel()
	}
	ctx.cancel = nil

	// Don't reset ctx.s!
	// We have a pool per server so the next time this ctx is used it
	// will be assigned the same value again.
	// ctx might still be in use for context.Done() and context.Err()
	// which are safe to use as they only use ctx.s and no other value.

	if ctx.timeoutResponse != nil {
		ctx.timeoutResponse.Reset()
	}

	if ctx.timeoutTimer != nil {
		stopTimer(ctx.timeoutTimer)
	}
}

type firstByteReader struct {
	c        net.Conn
	ch       byte
	byteRead bool
}

func (r *firstByteReader) reset() {
	r.c = nil
	r.ch = 0
	r.byteRead = false
}

func (r *firstByteReader) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	nn := 0
	if !r.byteRead {
		b[0] = r.ch
		b = b[1:]
		r.byteRead = true
		nn = 1
	}
	n, err := r.c.Read(b)
	return n + nn, err
}

// Error sets response body to the given message.
func (ctx *RequestContext) Error(msg string) {
	ctx.Response.Reset()
	ctx.WriteResponseString(msg)
}

// WriteResponseString sets response body to the given value.
func (ctx *RequestContext) WriteResponseString(body string) {
	ctx.Response.SetBodyString(body)
}

// WriteResponse sets response body to the given value.
func (ctx *RequestContext) WriteResponse(body []byte) {
	ctx.Response.SetBody(body)
}

// Context returns request-bound context.
func (ctx *RequestContext) Context() context.Context {
	return ctx.ctx
}

// Body returns request body.
func (ctx *RequestContext) Body() []byte {
	return ctx.Request.Body()
}

// String returns unique string representation of the ctx.
//
// The returned value may be useful for logging.
func (ctx *RequestContext) String() string {
	return fmt.Sprintf("#%08X - %s<->%s", ctx.ID(), ctx.LocalAddr(), ctx.RemoteAddr())
}

var zeroTCPAddr = &net.TCPAddr{
	IP: net.IPv4zero,
}

// RemoteAddr returns client address for the given request.
//
// Always returns non-nil result.
func (ctx *RequestContext) RemoteAddr() net.Addr {
	if ctx.remoteAddr != nil {
		return ctx.remoteAddr
	}
	if ctx.c == nil {
		return zeroTCPAddr
	}
	addr := ctx.c.RemoteAddr()
	if addr == nil {
		return zeroTCPAddr
	}
	return addr
}

// LocalAddr returns server address for the given request.
//
// Always returns non-nil result.
func (ctx *RequestContext) LocalAddr() net.Addr {
	if ctx.c == nil {
		return zeroTCPAddr
	}
	addr := ctx.c.LocalAddr()
	if addr == nil {
		return zeroTCPAddr
	}
	return addr
}

// ID returns unique ID of the request.
func (ctx *RequestContext) ID() uint64 {
	return ctx.connID
}
