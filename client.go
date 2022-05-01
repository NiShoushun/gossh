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
		ClientVersion:     "SSH-2.0-GOSSH",
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

// Dial 发送 direct-tcpip 通道建立请求，通过已经建立的 SSH 连接，连接至远程服务端。
// netType 为网络类型 tcp、tcp4、tcp6 以及 unix 之一。
// 一个经典的应用就是 Open-SSH 的 ssh -L 端口转发
func (client *SSHClient) Dial(netType, addr string) (net.Conn, error) {
	return client.c.Dial(netType, addr)
}

// Listen 发送 tcpip-forward 通道建立请求，通过本次建立的 SSH 信道，任何对目标地址端口的访问都将被转发至本地，
// 从而监听远程系统端口。
// netType 为网络类型 tcp、tcp4、tcp6 以及 unix 之一。
// 一个最经典的应用就是 Open-SSH 的 ssh -R 端口转发，发送至远程目标端口的数据都将被转发至返回的监听器。
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
	Rand           io.Reader
	RekeyThreshold uint64
	KeyExchanges   []string
	Ciphers        []string
	MACs           []string

	User              string
	Auth              []AuthMethod
	HostKeyCallback   HostKeyCallback
	BannerCallback    BannerCallback
	ClientVersion     string
	HostKeyAlgorithms []string
	Timeout           time.Duration
}
