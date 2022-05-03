package gossh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

type Director struct {
	client    *SSHClient
	NewConnCb NewConnCallback
}

type NewConnCallback func(conn net.Conn) (net.Conn, error)

// BindConnTo 与 BindConnToWithBuffer 相同，但使用默认的 buf
func (d *Director) BindConnTo(lconn net.Conn, netType, addr string, rctx, wctx context.Context) error {
	return d.BindConnToWithBuffer(lconn, netType, addr, 0, rctx, wctx)
}

// BindConnToWithBuffer 通过已经建立的 SSH 连接，打开并连接至远程端口，并与给定的流之间进行数据复制，复制时使用给定的 bufSize。
// rctx 用于控制远程连接至本地连接的数据传输的 Deadline 以及 cancelFunc 终止控制；
// wctx 用于控制本地连接至远程连接的数据传输的 Deadline 以及 cancelFunc 终止控制；
// 如果出现 IO 问题而不是 context 取消执行而导致的错误，双方网络连接将会被关闭
// 返回 error 不为 nil 表明远程网络连接未成功建立
func (d *Director) BindConnToWithBuffer(lconn net.Conn, netType, addr string, bufSize int, rctx, wctx context.Context) error {
	remoteConn, err := d.client.Dial(netType, addr)
	if err != nil {
		return err
	}
	d.bindConnsWithBuffer(lconn, remoteConn, bufSize, rctx, wctx)
	return nil
}

// BindTcpConnToWithBuffer 与 BindConnToWithBuffer 相似，但只能用于 TCP 连接。
// origin 表示源地址以及端口，如果 origin 为 nil, 将被零值（0.0.0.0:0）取代；
// to 表示要发送至远程服务器的地址以及端口；
// 返回 error 不为 nil 表明网络连接未成功建立。
func (d *Director) BindTcpConnToWithBuffer(lconn net.Conn, origin, to *net.TCPAddr, bufSize int, rctx, wctx context.Context) error {
	remoteConn, err := d.client.DialTCP("tcp", origin, to)
	if err != nil {
		return err
	}
	d.bindConnsWithBuffer(lconn, remoteConn, bufSize, rctx, wctx)
	return nil
}

// BindTcpConnTo 与 BindConnToWithBuffer 相同，但使用默认的 buf
func (d *Director) BindTcpConnTo(lconn net.Conn, origin, to *net.TCPAddr, rctx, wctx context.Context) error {
	return d.BindTcpConnToWithBuffer(lconn, origin, to, 0, rctx, wctx)

}

// 复制双方连接的数据流，如果出现 IO 问题而不是 context 取消执行而导致的错误，双方连接将会被关闭
func (d *Director) bindConnsWithBuffer(lconn net.Conn, rconn net.Conn, bufSize int, rctx, wctx context.Context) {
	var readBuf []byte = nil
	var writeBuf []byte = nil
	if bufSize > 0 {
		readBuf = make([]byte, bufSize)
		writeBuf = make([]byte, bufSize)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	// 开始并发传输数据 ：l->remote，当任何一方连接中断导致 Copy 函数返回，都要关闭两方连接
	go func() {
		CopyBufferWithContext(rconn, lconn, writeBuf, wctx)
		lconn.Close()
		rconn.Close()
		wg.Done()
	}()

	// 开始并发传输数据 ：remote->l，当任何一方连接中断导致 Copy 函数返回，都要关闭两方连接
	go func() {
		CopyBufferWithContext(lconn, rconn, readBuf, rctx)
		lconn.Close()
		rconn.Close()
		wg.Done()
	}()
	wg.Wait()
}

// RedirectToWithBuffer 通过传入的网络监听器接受网络连接，并尝试通过 direct-tcpip 信道打开一个远程端口并开始双向地复制数据。
// 将会阻塞，直至 Listener.Accept 返回的 err 不为 nil。
// 通过传入 Context 来控制 Deadline、终止监听以及终止流的复制。
func (d *Director) RedirectToWithBuffer(listener net.Listener, netType, addr string, bufSize int, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			lconn, err := listener.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(lconn.RemoteAddr().String())
			go func() {
				rconn, err := d.client.Dial(netType, addr)
				if err != nil {
					return
				}
				var readBuf []byte = nil
				var writeBuf []byte = nil
				if bufSize > 0 {
					readBuf = make([]byte, bufSize)
					writeBuf = make([]byte, bufSize)
				}
				// 开始并发传输数据 ：l->remote，当任何一方连接中断导致 Copy 函数返回，都要关闭两方连接
				go func() {
					CopyBufferWithContext(rconn, lconn, readBuf, ctx)
					fmt.Println("l->r")
					lconn.Close()
					rconn.Close()
				}()

				// 开始并发传输数据 ：remote->l，当任何一方连接中断导致 Copy 函数返回，都要关闭两方连接
				go func() {
					CopyBufferWithContext(lconn, rconn, writeBuf, ctx)
					fmt.Println("r->l")
					lconn.Close()
					rconn.Close()
				}()
			}()
		}
	}
}

// RedirectTo 与 RedirectToWithBuffer 作用相同，但使用默认的 buffer size
func (d *Director) RedirectTo(localL net.Listener, netType, toAddr string, ctx context.Context) {
	d.RedirectToWithBuffer(localL, netType, toAddr, 0, ctx)
}

// DirectTcpToWithBuffer 通过传入的网络监听器接受网络连接，并尝试通过 direct-tcpip 信道打开一个远程端口并开始双向地复制数据。
// 将会阻塞，直至 Listener.Accept 返回的 err 不为 nil。
// 通过传入 Context 来控制 Deadline、终止监听以及终止流的复制。
func (d *Director) DirectTcpToWithBuffer(listener net.Listener, to *net.TCPAddr, bufSize int, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			lconn, err := listener.Accept()
			if err != nil {
				return
			}

			if d.NewConnCb != nil {
				if transFormedConn, err := d.NewConnCb(lconn); err != nil {
					return
				} else {
					lconn = transFormedConn
				}
			}

			go func() {
				select {
				case <-ctx.Done():
					return
				default:
					origin, err := net.ResolveTCPAddr(lconn.RemoteAddr().Network(), lconn.LocalAddr().String())
					fmt.Println(origin)
					if err != nil {
						return
					}
					d.BindTcpConnToWithBuffer(lconn, origin, to, bufSize, ctx, ctx)
				}
			}()
		}
	}
}

// RedirectTcpTo 与 RedirectTcpToWithBuffer 作用相同，但使用默认的 buffer size
func (d *Director) RedirectTcpTo(localL net.Listener, netType, addr string, ctx context.Context) {
	d.RedirectToWithBuffer(localL, netType, addr, 0, ctx)
}

var errInvalidWrite = errors.New("invalid write result")

// CopyBufferWithContext 导出的 io.copyBuffer 函数，可传入 Context 对应的 cancelFunc 来终止流之间的复制，并返回 nil error
func CopyBufferWithContext(dst io.Writer, src io.Reader, buf []byte, ctx context.Context) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	for {
		select {
		case <-ctx.Done():
			return written, interruptedErr
		default:
			nr, er := src.Read(buf)
			if nr > 0 {
				os.Stdout.Write(buf[0:nr])
				nw, ew := dst.Write(buf[0:nr])
				if nw < 0 || nr < nw {
					nw = 0
					if ew == nil {
						ew = errInvalidWrite
					}
				}
				written += int64(nw)
				if ew != nil {
					err = ew
					goto ret
				}
				if nr != nw {
					err = io.ErrShortWrite
					goto ret
				}
			}
			if er != nil {
				if er != io.EOF {
					err = er
				}
				goto ret
			}
		}
	}
ret:
	return written, err
}

var interruptedErr = errors.New("interrupted")
