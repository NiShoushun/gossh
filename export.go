package gossh

import (
	"golang.org/x/crypto/ssh"
	"net"
)

type BannerCallback func(message string) error

type HostKeyCallback func(hostname string, remote net.Addr, key PublicKey) error

// RetryableAuthMethod 是其他 auth 方法的装饰器，使它们能够在考虑 AuthMethod 本身失败之前重试到 maxTries。如果 maxTries <= 0，将无限期重试
func RetryableAuthMethod(auth AuthMethod, maxTries int) AuthMethod {
	return ssh.RetryableAuthMethod(auth, maxTries)
}

// WrapBannerCallback WrapHostKeyCallback 将 BannerCallback 转化为 ssh 包可接受参数类型
func WrapBannerCallback(callback BannerCallback) func(message string) error {
	if callback == nil {
		return nil
	}
	return func(message string) error {
		return callback(message)
	}
}

// WrapHostKeyCallback 将 HostKeyCallback 转化为 ssh 包可接受参数类型
func WrapHostKeyCallback(callback HostKeyCallback) func(hostname string, remote net.Addr, key ssh.PublicKey) error {
	if callback == nil {
		return nil
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return callback(hostname, remote, key)
	}
}

// KeyboardInteractiveChallenge 应该打印服务端问题，可选择禁用回显（例如密码），并返回所有答案。
// KeyboardInteractiveChallenge 可以在单个会话中被多次调用。
// 认证成功后，服务器可以发送一个不带问题的挑战，应打印名称和指令消息。
// RFC 4256 第 3.3 节详细说明了 UI 在 CLI 和 GUI 环境中的行为方式。
type KeyboardInteractiveChallenge func(name, instruction string, questions []string, echos []bool) (answers []string, err error)

// KeyboardInteractive 返回一个 AuthMethod
func KeyboardInteractive(challenge KeyboardInteractiveChallenge) AuthMethod {
	return ssh.KeyboardInteractive(ssh.KeyboardInteractiveChallenge(challenge))
}

type NewChannel interface {
	ssh.NewChannel
}

type Channel interface {
	ssh.Channel
}

type PublicKey interface {
	ssh.PublicKey
}

type AuthMethod interface {
	ssh.AuthMethod
}

func WrapAuthMethodSlice(methods []AuthMethod) []ssh.AuthMethod {
	sa := make([]ssh.AuthMethod, 0)
	for _, method := range methods {
		sa = append(sa, method)
	}
	return sa
}
