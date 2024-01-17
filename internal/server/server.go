package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RequestHandler must process incoming requests.
//
// RequestHandler must call ctx.TimeoutError() before returning
// if it keeps references to ctx and/or its members after the return.
// Consider wrapping RequestHandler into TimeoutHandler if response time
// must be limited.
type RequestHandler func(ctx *RequestContext)

type ServeHandler func(c net.Conn) error

// Server implements TCP server.
//
// Default Server settings should satisfy the majority of Server users.
// Adjust Server settings only if you really understand the consequences.
//
// It is forbidden copying Server instances. Create new Server instances
// instead.
//
// It is safe to call Server methods from concurrently running goroutines.
type Server struct {
	noCopy noCopy

	// Handler for processing incoming requests.
	//
	// Take into account that no `panic` recovery is done by `server` (thus any `panic` will take down the entire server).
	// Instead, the user should use `recover` to handle these situations.
	Handler RequestHandler

	// ErrorHandler for returning a response in case of an error while receiving or parsing the request.
	//
	// The following is a non-exhaustive list of errors that can be expected as argument:
	//   * io.EOF
	//   * io.ErrUnexpectedEOF
	ErrorHandler func(ctx *RequestContext, err error)

	// The maximum number of concurrent connections the server may serve.
	//
	// DefaultConcurrency is used if not set.
	//
	// Concurrency only works if you call Serve once
	Concurrency int

	// Per-connection buffer size for requests' reading.
	//
	// Default buffer size is used if not set.
	ReadBufferSize int

	// Per-connection buffer size for responses' writing.
	//
	// Default buffer size is used if not set.
	WriteBufferSize int

	// ReadTimeout is the amount of time allowed to read
	// the full request including body. The connection's read
	// deadline is reset when the connection opens, or for
	// keep-alive connections after the first byte has been read.
	//
	// By default request read timeout is unlimited.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset after the request handler
	// has returned.
	//
	// By default response write timeout is unlimited.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alive is enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used.
	IdleTimeout time.Duration

	// Maximum number of concurrent client connections allowed per IP.
	//
	// By default unlimited number of concurrent connections
	// may be established to the server from a single IP address.
	MaxConnsPerIP int

	// MaxIdleWorkerDuration is the maximum idle time of a single worker in the underlying
	// worker pool of the Server. Idle workers beyond this time will be cleared.
	MaxIdleWorkerDuration time.Duration

	// Maximum request body size.
	//
	// The server rejects requests with bodies exceeding this limit.
	//
	// Request body size is limited by DefaultMaxRequestBodySize by default.
	MaxRequestBodySize int64

	// Global context for all requests.
	globalContext context.Context
	globalCancel  context.CancelFunc

	concurrency      uint32
	concurrencyCh    chan struct{}
	perIPConnCounter perIPConnCounter

	ctxPool    sync.Pool
	readerPool sync.Pool
	writerPool sync.Pool

	ln net.Listener

	mu   sync.Mutex
	open int32
	stop int32
	done chan struct{}
}

// ErrNothingRead is returned when a remote closed it or because of a read timeout.
type ErrNothingRead struct {
	error
}

// DefaultConcurrency is the maximum number of concurrent connections
// the Server may serve by default (i.e. if Server.Concurrency isn't set).
const DefaultConcurrency = 256 * 1024

// ServeContext serves incoming connections from the given context and listener.
func (s *Server) ServeContext(ctx context.Context, ln net.Listener) error {
	s.setGlobalContext(ctx)

	return s.Serve(ln)
}

// Serve serves incoming connections from the given listener.
//
// Serve blocks until the given listener returns permanent error.
func (s *Server) Serve(ln net.Listener) error {
	var lastOverflowErrorTime time.Time
	var lastPerIPErrorTime time.Time

	maxWorkersCount := s.getConcurrency()

	s.mu.Lock()
	s.ln = ln
	if s.done == nil {
		s.done = make(chan struct{})
	}
	if s.concurrencyCh == nil {
		s.concurrencyCh = make(chan struct{}, maxWorkersCount)
	}
	s.mu.Unlock()

	wp := &workerPool{
		WorkerFunc:            s.serveConn,
		MaxWorkersCount:       maxWorkersCount,
		MaxIdleWorkerDuration: s.MaxIdleWorkerDuration,
	}
	wp.Start()

	// Count our waiting to accept a connection as an open connection.
	// This way we can't get into any weird state where just after accepting
	// a connection Shutdown is called which reads open as 0 because it isn't
	// incremented yet.
	atomic.AddInt32(&s.open, 1)
	defer atomic.AddInt32(&s.open, -1)

	for {
		c, err := acceptConn(s, ln, &lastPerIPErrorTime)
		if err != nil {
			wp.Stop()
			if err == io.EOF {
				return nil
			}
			return err
		}
		atomic.AddInt32(&s.open, 1)
		if !wp.Serve(c) {
			atomic.AddInt32(&s.open, -1)
			s.writeFastError(c, "The connection cannot be served because Server.Concurrency limit exceeded")
			_ = c.Close()
			if time.Since(lastOverflowErrorTime) > time.Minute {
				lastOverflowErrorTime = time.Now()
			}
		}
	}
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown works by first closing all open listeners and then waiting indefinitely for all connections
// to return to idle and then shut down.
//
// When Shutdown is called, Serve immediately return nil.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Shutdown does not close keepalive connections so it's recommended to set ReadTimeout and IdleTimeout to something else than 0.
func (s *Server) Shutdown() error {
	return s.ShutdownWithContext(context.Background())
}

// ShutdownWithContext gracefully shuts down the server without interrupting any active connections.
// ShutdownWithContext works by first closing all open listeners and then waiting for all connections to return to idle
// or context timeout and then shut down.
//
// When ShutdownWithContext is called, Serve immediately return nil.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// ShutdownWithContext does not close keepalive connections so it's recommended to set ReadTimeout and IdleTimeout
// to something else than 0.
func (s *Server) ShutdownWithContext(ctx context.Context) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	atomic.StoreInt32(&s.stop, 1)
	defer atomic.StoreInt32(&s.stop, 0)

	if s.ln == nil {
		return nil
	}

	if err = s.ln.Close(); err != nil {
		return err
	}

	if s.done != nil {
		close(s.done)
	}

	// Closing the listener will make Serve() call Stop on the worker pool.
	// Setting .stop to 1 will make serveConn() break out of its loop.
	// Now we just have to wait until all workers are done or timeout.
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
END:
	for {

		if open := atomic.LoadInt32(&s.open); open == 0 {
			break
		}
		// This is not an optimal solution but using a sync.WaitGroup
		// here causes data races as it's hard to prevent Create() to be called
		// while Wait() is waiting.
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break END
		case <-ticker.C:
			continue
		}
	}

	s.done = nil
	s.ln = nil
	return err
}

func acceptConn(s *Server, ln net.Listener, lastPerIPErrorTime *time.Time) (net.Conn, error) {
	for {
		c, err := ln.Accept()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				time.Sleep(time.Second)
				continue
			}
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				return nil, err
			}
			return nil, io.EOF
		}

		if s.MaxConnsPerIP > 0 {
			pic := wrapPerIPConn(s, c)
			if pic == nil {
				if time.Since(*lastPerIPErrorTime) > time.Minute {
					*lastPerIPErrorTime = time.Now()
				}
				continue
			}
			c = pic
		}
		return c, nil
	}
}

func wrapPerIPConn(s *Server, c net.Conn) net.Conn {
	ip := getUint32IP(c)
	if ip == 0 {
		return c
	}
	n := s.perIPConnCounter.Register(ip)
	if n > s.MaxConnsPerIP {
		s.perIPConnCounter.Unregister(ip)
		s.writeFastError(c, "The number of connections from your ip exceeds MaxConnsPerIP")
		_ = c.Close()
		return nil
	}
	return acquirePerIPConn(c, ip, &s.perIPConnCounter)
}

// GetCurrentConcurrency returns a number of currently served
// connections.
//
// This function is intended be used by monitoring systems.
func (s *Server) GetCurrentConcurrency() uint32 {
	return atomic.LoadUint32(&s.concurrency)
}

// GetOpenConnectionsCount returns a number of opened connections.
//
// This function is intended be used by monitoring systems.
func (s *Server) GetOpenConnectionsCount() int32 {
	if atomic.LoadInt32(&s.stop) == 0 {
		// Decrement by one to avoid reporting the extra open value that gets
		// counted while the server is listening.
		return atomic.LoadInt32(&s.open) - 1
	}
	// This is not perfect, because s.stop could have changed to zero
	// before we load the value of s.open. However, in the common case
	// this avoids underreporting open connections by 1 during server shutdown.
	return atomic.LoadInt32(&s.open)
}

func (s *Server) getConcurrency() int {
	n := s.Concurrency
	if n <= 0 {
		n = DefaultConcurrency
	}
	return n
}

var globalConnID uint64

func nextConnID() uint64 {
	return atomic.AddUint64(&globalConnID, 1)
}

// DefaultMaxRequestBodySize is the maximum request body size the server
// reads by default.
//
// See Server.MaxRequestBodySize for details.
const DefaultMaxRequestBodySize = 4 * 1024 * 1024

var zeroTime time.Time

func (s *Server) idleTimeout() time.Duration {
	if s.IdleTimeout != 0 {
		return s.IdleTimeout
	}
	return s.ReadTimeout
}

func (s *Server) serveConnCleanup() {
	atomic.AddInt32(&s.open, -1)
	atomic.AddUint32(&s.concurrency, ^uint32(0))
}

func (s *Server) serveConn(c net.Conn) (err error) {
	defer s.serveConnCleanup()
	atomic.AddUint32(&s.concurrency, 1)

	connID := nextConnID()
	maxRequestBodySize := s.MaxRequestBodySize
	if maxRequestBodySize <= 0 {
		maxRequestBodySize = DefaultMaxRequestBodySize
	}
	writeTimeout := s.WriteTimeout
	previousWriteTimeout := time.Duration(0)

	ctx := s.acquireCtx(c)
	var (
		br *bufio.Reader
		bw *bufio.Writer

		timeoutResponse *Response
	)
	for {
		if br == nil {
			br = acquireReader(ctx)
		}

		if err == nil {
			if s.ReadTimeout > 0 {
				if err := c.SetReadDeadline(time.Now().Add(s.ReadTimeout)); err != nil {
					break
				}
			} else if s.IdleTimeout > 0 {
				// If this was an idle connection and the server has an IdleTimeout but
				// no ReadTimeout then we should remove the ReadTimeout.
				if err := c.SetReadDeadline(zeroTime); err != nil {
					break
				}
			}

			if err == nil {
				err = ctx.Request.ReadBody(br, maxRequestBodySize)
			}

			if br.Buffered() == 0 || err != nil {
				releaseReader(s, br)
				br = nil
			}
		}

		if err != nil {
			if err == io.EOF {
				err = nil
			}

			var nr ErrNothingRead
			if errors.As(err, &nr) {
				err = nr.error
			}

			if err != nil {
				bw = s.writeErrorResponse(bw, ctx, err)
			}
			break
		}

		ctx.connID = connID
		s.Handler(ctx)

		timeoutResponse = ctx.timeoutResponse
		if timeoutResponse != nil {
			// Acquire a new ctx because the old one will still be in use by the timeout out handler.
			ctx = s.acquireCtx(c)
			timeoutResponse.CopyTo(&ctx.Response)
		}

		if writeTimeout > 0 {
			if err := c.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				panic(fmt.Sprintf("BUG: error in SetWriteDeadline(%v): %v", writeTimeout, err))
			}
			previousWriteTimeout = writeTimeout
		} else if previousWriteTimeout > 0 {
			// We don't want a write timeout but we previously set one, remove it.
			if err := c.SetWriteDeadline(zeroTime); err != nil {
				panic(fmt.Sprintf("BUG: error in SetWriteDeadline(zeroTime): %v", err))
			}
			previousWriteTimeout = 0
		}

		if bw == nil {
			bw = acquireWriter(ctx)
		}
		if err = writeResponse(ctx, bw); err != nil {
			break
		}
		if err = bw.Flush(); err != nil {
			break
		}

		ctx.Request.Reset()
		ctx.Response.Reset()

		if atomic.LoadInt32(&s.stop) == 1 {
			err = nil
			break
		}
	}

	if br != nil {
		releaseReader(s, br)
	}
	if bw != nil {
		releaseWriter(s, bw)
	}

	s.releaseCtx(ctx)

	return
}

func (s *Server) writeFastError(w io.Writer, msg string) {
	_, _ = w.Write([]byte(msg))
}

func defaultErrorHandler(ctx *RequestContext, err error) {
	var netErr *net.OpError
	if errors.As(err, &netErr) && netErr.Timeout() {
		ctx.Error("Request timeout")
	}
}

func (s *Server) writeErrorResponse(bw *bufio.Writer, ctx *RequestContext, err error) *bufio.Writer {
	errorHandler := defaultErrorHandler
	if s.ErrorHandler != nil {
		errorHandler = s.ErrorHandler
	}

	errorHandler(ctx, err)

	if bw == nil {
		bw = acquireWriter(ctx)
	}

	_ = writeResponse(ctx, bw)
	ctx.Response.Reset()
	_ = bw.Flush()

	return bw
}

func (s *Server) acquireCtx(c net.Conn) (ctx *RequestContext) {
	v := s.ctxPool.Get()
	if v == nil {
		ctx = new(RequestContext)
		ctx.s = s
	} else {
		ctx = v.(*RequestContext)
	}

	ctx.c = c

	ctx.ctx, ctx.cancel = context.WithCancel(s.globalContext)

	return ctx
}

func (s *Server) releaseCtx(ctx *RequestContext) {
	if ctx.timeoutResponse != nil {
		panic("BUG: cannot release timed out RequestContext")
	}

	ctx.reset()
	s.ctxPool.Put(ctx)
}

func (s *Server) setGlobalContext(ctx context.Context) {
	s.mu.Lock()
	s.globalContext, s.globalCancel = context.WithCancel(ctx)
	s.mu.Unlock()
}

func writeResponse(ctx *RequestContext, w *bufio.Writer) error {
	if ctx.timeoutResponse != nil {
		return errors.New("cannot write timed out response")
	}
	err := ctx.Response.Write(w)

	return err
}

const (
	defaultReadBufferSize  = 4096
	defaultWriteBufferSize = 4096
)

func acquireReader(ctx *RequestContext) *bufio.Reader {
	v := ctx.s.readerPool.Get()
	if v == nil {
		n := ctx.s.ReadBufferSize
		if n <= 0 {
			n = defaultReadBufferSize
		}
		return bufio.NewReaderSize(ctx.c, n)
	}
	r := v.(*bufio.Reader)
	r.Reset(ctx.c)
	return r
}

func releaseReader(s *Server, r *bufio.Reader) {
	s.readerPool.Put(r)
}

func acquireWriter(ctx *RequestContext) *bufio.Writer {
	v := ctx.s.writerPool.Get()
	if v == nil {
		n := ctx.s.WriteBufferSize
		if n <= 0 {
			n = defaultWriteBufferSize
		}
		return bufio.NewWriterSize(ctx.c, n)
	}
	w := v.(*bufio.Writer)
	w.Reset(ctx.c)
	return w
}

func releaseWriter(s *Server, w *bufio.Writer) {
	s.writerPool.Put(w)
}
