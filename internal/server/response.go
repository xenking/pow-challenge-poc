package server

import (
	"bufio"
	"bytes"
	"net"
	"sync"
)

// Response represents TCP response.
//
// It is forbidden copying Response instances. Create new instances
// and use CopyTo instead.
//
// Response instance MUST NOT be used from concurrently running goroutines.
type Response struct {
	noCopy noCopy

	w    responseBodyWriter
	body *bytes.Buffer

	// Remote TCPAddr from concurrently net.Conn.
	raddr net.Addr
	// Local TCPAddr from concurrently net.Conn.
	laddr net.Addr
}

type responseBodyWriter struct {
	r *Response
}

func (w *responseBodyWriter) Write(p []byte) (int, error) {
	w.r.AppendBody(p)
	return len(p), nil
}

// CopyTo copies resp contents to dst except of body stream.
func (resp *Response) CopyTo(dst *Response) {
	dst.Reset()
	dst.raddr = resp.raddr
	dst.laddr = resp.laddr
	dst.body.Write(resp.body.Bytes())
}

// Reset clears response contents.
func (resp *Response) Reset() {
	resp.raddr = nil
	resp.laddr = nil
	if resp.body != nil {
		resp.body.Reset()
	}
}

func (resp *Response) bodyBuffer() *bytes.Buffer {
	if resp.body == nil {
		resp.body = responseBodyPool.Get().(*bytes.Buffer)
	}
	return resp.body
}

var responseBodyPool = &sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// AppendBody appends p to response body.
//
// It is safe re-using p after the function returns.
func (resp *Response) AppendBody(p []byte) {
	resp.bodyBuffer().Write(p)
}

// Write writes response to w.
//
// Write doesn't flush response to w for performance reasons.
//
// See also WriteTo.
func (resp *Response) Write(w *bufio.Writer) error {
	body := resp.bodyBytes()
	if _, err := w.Write(body); err != nil {
		return err
	}
	return nil
}

func (resp *Response) bodyBytes() []byte {
	if resp.body == nil {
		return nil
	}
	return resp.body.Bytes()
}

// SetBodyString sets request body.
func (resp *Response) SetBodyString(body string) {
	bodyBuf := resp.bodyBuffer()
	bodyBuf.Reset()
	bodyBuf.WriteString(body)
	bodyBuf.WriteByte('\n')
}

// SetBody sets request body.
func (resp *Response) SetBody(body []byte) {
	bodyBuf := resp.bodyBuffer()
	bodyBuf.Reset()
	bodyBuf.Write(body)
	bodyBuf.WriteByte('\n')
}
