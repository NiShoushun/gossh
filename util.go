package gossh

import (
	"fmt"
	user2 "os/user"
	"path"
)

// DisplayBanner 简单地打印服务器发送的 banner 信息
func DisplayBanner(message string) error {
	fmt.Println(message)
	return nil
}

// PrivateKeyPath 获取给定用户的默认的 Open-SSH 私钥路径
func PrivateKeyPath(username string) (string, error) {
	user, err := user2.Lookup(username)
	if err != nil {
		return "", err
	}
	return path.Join(user.HomeDir, DefaultPrivateKeyPath), nil
}

// KnownHostsPath 获取给定用户的默认的 Open-SSH known_hosts 路径
func KnownHostsPath(username string) (string, error) {
	user, err := user2.Lookup(username)
	if err != nil {
		return "", err
	}
	return path.Join(user.HomeDir, DefaultKnownHosts), nil
}

// DefaultConfigAuthByPasswd 生成默认的配置，认证方式为密码认证。
// 不会主机公钥验证，需要后续添加 HostKeyCallback；
// user 为登陆用户名，password 为登陆密码
func DefaultConfigAuthByPasswd(user, password string) *Config {
	return &Config{
		User:            user,
		HostKeyCallback: IgnoreHostKey,
		Auth:            []AuthMethod{PasswordAuth(password)},
	}
}

// DefaultConfigAuthByAgent 生成默认的配置，认证方式为 ssh-agent 认证。
// 不会主机公钥验证，需要后续添加 HostKeyCallback；
// user 为登陆用户名
func DefaultConfigAuthByAgent(user string) (*Config, error) {
	auth, err := SSHAgentAuth()
	if err != nil {
		return nil, err
	}
	return &Config{
		User:            user,
		HostKeyCallback: IgnoreHostKey,
		Auth:            []AuthMethod{auth},
	}, nil
}

// DefaultConfigAuthByPrivateKey 生成默认的配置，认证方式为私钥认证。
// 不会主机公钥验证，需要后续添加 HostKeyCallback；
// user 为登陆用户名，keys 为多个私钥文件的内容
func DefaultConfigAuthByPrivateKey(user string, keys ...[]byte) (*Config, error) {
	auth, err := AuthByPrivateKeys(keys...)
	if err != nil {
		return nil, err
	}
	return &Config{
		User:            user,
		HostKeyCallback: IgnoreHostKey,
		Auth:            []AuthMethod{auth},
	}, nil
}

// DefaultConfigAuthByPrivateKeyFromPaths 生成默认的配置，认证方式为私钥认证。
// 不会主机公钥验证，需要后续添加 HostKeyCallback；
// user 为登陆用户名，keys 为多个私钥文件的路径
func DefaultConfigAuthByPrivateKeyFromPaths(user string, path ...string) (*Config, error) {
	auth, err := AuthByPrivateKeysFromPaths(path...)
	if err != nil {
		return nil, err
	}
	return &Config{
		User:            user,
		HostKeyCallback: IgnoreHostKey,
		Auth:            []AuthMethod{auth},
	}, nil
}

// CurrentUser 当前的的用户名
func CurrentUser() string {
	user, _ := user2.Current()
	return user.Username
}
