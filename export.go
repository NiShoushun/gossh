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
