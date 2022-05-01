package gossh

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// 本文件中的函数用于处理 ssh 认证功能

type KnownHostsChecker struct {
	files         []string
	Interactively bool
}

const (
	DefaultPrivateKeyPath = ".ssh/id_rsa"
	DefaultKnownHosts     = ".ssh/known_hosts"
)

// AuthByPrivateKeysFromPaths 从给定的文件中加载私钥并生成 Signer，并生成 ssh.AuthMethod 认证方法.
func AuthByPrivateKeysFromPaths(files ...string) (ssh.AuthMethod, error) {
	var signers []ssh.Signer
	for _, file := range files {
		// 读取密钥文件
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}
		// 解析ssh 私钥
		signer, err := ssh.ParsePrivateKey(bytes)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}
	return ssh.PublicKeys(signers...), nil
}

func AuthByPrivateKeys(keys ...[]byte) (ssh.AuthMethod, error) {
	var signers []ssh.Signer
	for _, key := range keys {
		// 读取密钥文件

		// 解析ssh 私钥
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		signers = append(signers, signer)
	}
	return ssh.PublicKeys(signers...), nil
}

// ReadPasswordAuth 从标准输入中获取输入密码进行认证
// prompt 为输入前的字符提示；
func ReadPasswordAuth(prompt ...string) (AuthMethod, error) {
	for _, s := range prompt {
		fmt.Print(s)
	}
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("read passwd failed: %s", err)
	}
	return ssh.Password(string(password)), nil
}

// PasswordAuth 由给定的密码进行认证
func PasswordAuth(passwd string) AuthMethod {
	return ssh.Password(passwd)
}

// SSHAgentAuth ssh-agent 身份验证
func SSHAgentAuth() (AuthMethod, error) {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), nil
	}
	return nil, err
}

// NewFixHostKeyCallback 用于固定主机公钥的主机验证方式
func NewFixHostKeyCallback(key []byte) (func(hostname string, remote net.Addr, key ssh.PublicKey) error, error) {
	pubKey, err := ssh.ParsePublicKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.FixedHostKey(pubKey), nil
}

// IgnoreHostKey 忽略主机公钥，总是返回 nil error
func IgnoreHostKey(hostname string, remote net.Addr, key PublicKey) error {
	return nil
}

// NewKnownHostCallback 生成一个known_host callback
func NewKnownHostCallback(interactively bool, files ...string) func(hostname string, remote net.Addr, key PublicKey) error {
	return (&KnownHostsChecker{
		files:         files,
		Interactively: interactively,
	}).KnownHostsCheck
}

// KnownHostsCheck 对 hostname 、remote 与 key 进行 known_hosts 匹配。
// 如果接收器的 interactively 为 true，将会以交互式方式询问是否接受此次连接，并将
func (kw KnownHostsChecker) KnownHostsCheck(hostname string, remote net.Addr, key PublicKey) error {
	if kw.files == nil || len(kw.files) == 0 {
		return errors.New("no known_hosts file given")
	}

	callback, err := knownhosts.New(kw.files...)
	if err != nil {
		if rErr, ok := err.(*knownhosts.RevokedError); ok {
			if kw.Interactively {
				fmt.Println("revoked key found")
			}
			return rErr
		}
		return err
	}

	err = callback(hostname, remote, key)
	if err != nil {
		return err
	}

	if !kw.Interactively {
		return nil
	}

	fmt.Printf("No host key found in %s, continue? [yes/NO]\r\n", kw.files)
	answer := "no"
	_, err = fmt.Scan(&answer)
	if err != nil {
		return err
	}
	if strings.ToLower(answer) == "yes" {
		answer = "no"
		fmt.Printf("Append %s@%s 's host key into '%s'? [YES/no] ", hostname, remote.String(), kw.files[0])
		_, err = fmt.Scan(&answer)
		if err != nil {
			return err
		}
		if strings.ToLower(answer) == "no" {
			return nil
		}
		if answer == "yes" {
			file, err := os.OpenFile(kw.files[0], os.O_WRONLY|os.O_APPEND, 0600)
			line := knownhosts.Line([]string{remote.String()}, key)
			_, err = file.WriteString(line + "\n")
			if err != nil {
				fmt.Printf("Add host key to %s failed: %s\r\n", kw.files[0], err)
			}
		}
		return nil
	} else {
		return errors.New("unknown host")
	}
}

//// 在known_hosts文件中查找主机名对应的公钥。如果存在对应host的记录，则 error 为空
//// 倘若有host对应多条记录，则返回最后一条
//// 常见的 known_hosts 记录格式：host cipher host-key
//func (kw *KnownHostsChecker) findHostKey(host string) (ssh.PublicKey, error) {
//	file, err := os.OpenFile(kw.Path, os.O_RDONLY, 0600)
//	if err != nil {
//		return nil, err
//	}
//	defer file.Close()
//	scanner := bufio.NewScanner(file)
//	var hostKey ssh.PublicKey
//	// 遍历所有记录，找到最后一条与host相符合的记录
//	for scanner.Scan() {
//		line := scanner.Bytes()
//		kw.Logger.Debugf("check known_host record: %s\n", string(line))
//		// 通过 ParseAuthorizedKey 方式来解析 known_hosts 文件
//		//fields := strings.Split(line, " ")
//		//// 一条正常记录应该有三段
//		//if len(fields) != 3 {
//		//	continue
//		//}
//		//// 第一个为 hostname
//		//if host == fields[0] {
//		//	lastHostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(fields[1] + " " + fields[2]))
//		//	if err != nil {
//		//		continue
//		//	}
//		//	hostKey = lastHostKey
//		//}
//
//		_, hosts, pubKey, _, _, err := ssh.ParseKnownHosts(line)
//		if err != nil {
//			continue
//		}
//		for _, h := range hosts {
//			if host == h {
//				hostKey = pubKey
//			}
//		}
//	}
//	return hostKey, nil
//}
