package server

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"time"
)

// Request represents TCP request.
//
// It is forbidden copying Request instances. Create new instances
// and use CopyTo instead.
//
// Request instance MUST NOT be used from concurrently running goroutines.
type Request struct {
	noCopy noCopy

	w    requestBodyWriter
	body *bytes.Buffer

	// Request timeout. Usually set by DoDeadline or DoTimeout
	// if <= 0, means not set
	timeout time.Duration
}

type requestBodyWriter struct {
	r *Request
}

func (w *requestBodyWriter) Write(p []byte) (int, error) {
	w.r.AppendBody(p)
	return len(p), nil
}

// CopyTo copies req contents to dst.
func (req *Request) CopyTo(dst *Request) {
	dst.Reset()
	dst.timeout = req.timeout
	dst.body.Write(req.body.Bytes())
}

// Reset clears request contents.
func (req *Request) Reset() {
	req.timeout = 0
	if req.body != nil {
		req.body.Reset()
	}
}

// ReadBody reads request body from the given r, limiting the body size.
func (req *Request) ReadBody(r *bufio.Reader, maxBodySize int64) error {
	bodyBuf := req.bodyBuffer()
	bodyBuf.Reset()

	lr := io.LimitReader(r, maxBodySize)

	reader := bufio.NewReader(lr)
	raw, err := reader.ReadBytes('\n')
	if err != nil {
		req.Reset()
		return err
	}

	// Remove trailing newline
	raw = raw[:len(raw)-1]

	_, err = bodyBuf.Write(raw)
	if err != nil {
		req.Reset()
		return err
	}

	return nil
}

func (req *Request) bodyBuffer() *bytes.Buffer {
	if req.body == nil {
		req.body = requestBodyPool.Get().(*bytes.Buffer)
	}
	return req.body
}

var requestBodyPool = &sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// AppendBody appends p to request body.
//
// It is safe re-using p after the function returns.
func (req *Request) AppendBody(p []byte) {
	req.bodyBuffer().Write(p)
}

// Write writes request to w.
//
// Write doesn't flush request to w for performance reasons.
//
// See also WriteTo.
func (req *Request) Write(w *bufio.Writer) error {
	body := req.bodyBytes()
	_, err := w.Write(body)
	return err
}

func (req *Request) bodyBytes() []byte {
	if req.body == nil {
		return nil
	}
	return req.body.Bytes()
}

func (req *Request) Body() []byte {
	return req.body.Bytes()
}
