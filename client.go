package gossh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"sync"
	"time"
)

// SSHClient 对 ssh.Client 的一层包装
type SSHClient struct {
	c *ssh.Client
	ssh.Conn
	sync.Mutex
}

// Connect 使用提供的配置选项与目标建立 SSH 连接
func Connect(addr string, config *Config) (*SSHClient, error) {
	if config == nil {
		return nil, errors.New("invalid config")
	}

	if config.ClientVersion == "" {
		config.ClientVersion = "SSH-2.0-GoSSH"
	}

	clientConfig := &ssh.ClientConfig{
		Config: ssh.Config{
			Rand:           config.Rand,
			RekeyThreshold: config.RekeyThreshold,
			KeyExchanges:   config.KeyExchanges,
			Ciphers:        config.Ciphers,
			MACs:           config.MACs,
		},
		User:              config.User,
		Auth:              WrapAuthMethodSlice(config.Auth),
		HostKeyCallback:   WrapHostKeyCallback(config.HostKeyCallback),
		BannerCallback:    WrapBannerCallback(config.BannerCallback),
		ClientVersion:     config.ClientVersion,
		HostKeyAlgorithms: config.HostKeyAlgorithms,
		Timeout:           15 * time.Second,
	}
	cli, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, err
	}

	return &SSHClient{
		c:     cli,
		Conn:  cli.Conn,
		Mutex: sync.Mutex{},
	}, err
}

// OpenSession 打开一个新的 session 通道
func (client *SSHClient) OpenSession() (*Session, error) {
	sess, err := client.c.NewSession()
	if err != nil {
		return nil, err
	}
	return &Session{
		sess:  sess,
		Mutex: sync.Mutex{},
	}, nil
}

// OpenChannel 请求建立一个新的 ssh 通道
func (client *SSHClient) OpenChannel(name string, extraData []byte) (Channel, <-chan *ssh.Request, error) {
	return client.Conn.OpenChannel(name, extraData)
}

// Dial 发送 direct-tcpip 通道建立请求，通过已经建立的 SSH 连接，连接至远程端口。
// netType 为网络类型 tcp、tcp4、tcp6 以及 unix 之一。
// 一个经典的应用就是 Open-SSH 的 ssh -L 端口转发
func (client *SSHClient) Dial(netType, addr string) (net.Conn, error) {
	return client.c.Dial(netType, addr)
}

// BindConnToRemote 与 BindConnToRemoteBuffer 相同，但使用默认的 buf
func (client *SSHClient) BindConnToRemote(lconn io.ReadWriteCloser, netType, addr string) (context.CancelFunc, error) {
	return client.BindConnToRemoteBuffer(lconn, netType, addr, nil)
}

// BindConnToRemoteBuffer 通过已经建立的 SSH 连接，连接至远程端口，并与给定的流之间进行数据复制，复制时使用给定的 buf 作为buffer。
// 通过返回的 CancelFunc 终止流的复制。
// 返回 error 不为 nil 表明网络连接未成功建立
func (client *SSHClient) BindConnToRemoteBuffer(lconn io.ReadWriteCloser, netType, addr string, buf []byte) (context.CancelFunc, error) {
	remoteConn, err := client.c.Dial(netType, addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	go CopyBufferWithContext(remoteConn, lconn, buf, ctx)
	go CopyBufferWithContext(lconn, remoteConn, buf, ctx)
	return cancel, nil
}

// Listen 发送 tcpip-forward 通道建立请求，通过本次建立的 SSH 信道，任何对目标地址端口的访问都将被转发至本地，
// 从而监听远程系统端口。
// netType 为网络类型 tcp、tcp4、tcp6 以及 unix 之一。
// 一个最经典的应用就是 Open-SSH 的 ssh -R 端口转发，发送至远程目标端口的连接与数据都将被转发至返回的监听器。
func (client *SSHClient) Listen(netType, addr string) (net.Listener, error) {
	return client.c.Listen(netType, addr)
}

// ListenTcp 类似于 Listen ，但是监听远程系统的 Tcp 端口，返回监听器，
func (client *SSHClient) ListenTcp(laddr *net.TCPAddr) (net.Listener, error) {
	return client.c.ListenTCP(laddr)
}

// ListenUnix 类似于 Listen ，监听远程 unix 系统的 unix socket
func (client *SSHClient) ListenUnix(socketPath string) (net.Listener, error) {
	return client.c.ListenUnix(socketPath)
}

// Config ssh 包下的 ClientConfig 的包装
type Config struct {
	Rand           io.Reader // 随机数源
	RekeyThreshold uint64    //
	KeyExchanges   []string  // 密钥交换算法算法
	Ciphers        []string  // 加密算法
	MACs           []string  // 消息摘要算法

	User              string          // 登陆用户
	Auth              []AuthMethod    // 身份验证方法列表
	HostKeyCallback   HostKeyCallback // 服务端主机公钥验证
	BannerCallback    BannerCallback  // 身份认证前对服务端发送的 Banner 信息的处理。注意，并不是所有的服务端都会发送该信息
	ClientVersion     string          // 必须以 'SSH-1.0-' 或者 'SSH-2.0-' 开头，如果为空，将被替换为 'SSH-2.0-GoSSH'
	HostKeyAlgorithms []string
	Timeout           time.Duration // 建立 tcp 连接超时时间
}

var errInvalidWrite = errors.New("invalid write result")

// CopyBufferWithContext 导出的 io.copyBuffer 函数，可传入 context 来终止流之间的复制
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
			return
		default:
			nr, er := src.Read(buf)
			if nr > 0 {
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
