package gossh

import (
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

// Client 获取原始的 ssh.Client
func (client *SSHClient) Client() *ssh.Client {
	return client.c
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

// NewDirector 创建一个 Director
func (client *SSHClient) NewDirector() *Director {
	return &Director{
		client: client,
	}
}

// Dial 发送 direct-tcpip 通道建立请求，通过已经建立的 SSH 连接，连接至远程端口。
// netType 为网络类型 tcp、tcp4、tcp6 以及 unix 之一；
// addr 应为远程服务端可访问的网络接口。
// 一个经典的应用就是 Open-SSH 的 ssh -L 端口转发
func (client *SSHClient) Dial(netType, addr string) (net.Conn, error) {
	return client.c.Dial(netType, addr)
}

// DialTCP 发送 direct-tcpip 通道建立请求，通过已经建立的 SSH 连接，建立TCP连接至远程端口。
// netType 为网络类型 tcp、tcp4、tcp6 之一；
// laddr 表示 tcp 请求来源，如果为 nil，将使用 '0.0.0.0:0'；raddr 为远程服务端可访问的地址以及端口
func (client *SSHClient) DialTCP(netType string, laddr, raddr *net.TCPAddr) (net.Conn, error) {
	return client.c.DialTCP(netType, laddr, raddr)
}

// Listen 发送 tcpip-forward 通道建立请求，通过本次建立的 SSH 信道，任何对 SSH 服务器上目标地址端口的访问都将被转发至本地，
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
