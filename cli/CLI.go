package cli

import (
	"context"
	"fmt"
	"github.com/nishoushun/gossh"
	"gopkg.in/alecthomas/kingpin.v2"
	"net"
	"os"
	"runtime"
	"time"
)

// 本文件包含 CLI 子命令以及选项定义
var (
	user        = gossh.CurrentUser()
	defaultTerm = "xterm-256color"
)

func privateKeyPath() string {
	privateKey, _ := gossh.PrivateKeyPath(user)
	return privateKey
}

func knownHostsPath() string {
	knownHosts, _ := gossh.KnownHostsPath(user)
	return knownHosts
}

// connect 子命令 ，用于连接远程服务器
var (
	portFlag              = kingpin.Flag("port", "port of ssh host.").Short('p').Default("22").String()
	hostFlag              = kingpin.Flag("host", "the host address of ssh host, ip address or domain name.").Short('h').String()
	userFlag              = kingpin.Flag("user", "specified user to login.").Short('u').Default(user).String()
	envsFlag              = kingpin.Flag("env", "set environment.").Short('e').StringMap()
	displayBannerFlag     = kingpin.Flag("display-banner", "display server banner.").Default("false").Bool()
	priKeyFlag            = kingpin.Flag("private-key", "use specified private key file.").Short('k').Default(privateKeyPath()).String()
	useAgentFlag          = kingpin.Flag("ssh-agent", "use ssh-agent for authentication.").Short('a').Default("false").Bool()
	forcePasswdFlag       = kingpin.Flag("passwd", "force to use password.").Short('P').Default("false").Bool()
	knownHostsFlag        = kingpin.Flag("known-hosts", "use specified known hosts file.").Default(knownHostsPath()).String()
	timeoutFlag           = kingpin.Flag("timeout", "timeout for connection.").Short('t').Default("0s").Duration()
	termFlag              = kingpin.Flag("term", "use the given terminal-color mod to run the interactive command line or shell.").Short('z').Default(defaultTerm).String()
	keepAliveFlag         = kingpin.Flag("keep-alive", "send useless request to keep tcp connection alive.").Short('l').Default("true").Bool()
	keepAliveIntervalFlag = kingpin.Flag("keep-alive-interval", "set the interval of keepalive request.").Short('i').Default("60s").Duration()

	ignoreKnownHostsFlag = kingpin.Flag("ignore-host-key", "do not check the server's host key.").Default("false").Bool()

	cipherFlags      = kingpin.Flag("cipher", "choose cipher algorithm").Strings()
	keyExchangeFlags = kingpin.Flag("key-exchange", "choose key exchange algorithm").Strings()
	macFlags         = kingpin.Flag("mac", "choose message authentication code algorithm").Strings()
	randFlag         = kingpin.Flag("rand", "choose a src to read random bytes").File()
)

var (
	// execCmd 连接后 exec子命令，用于执行命令
	execCmd    = kingpin.Command("exec", "execute command.") // execCmd exec 子命令 用于连接之后执行指令
	ptyFlag    = execCmd.Flag("pty", "request a pty before run command.").Short('y').Default("false").Bool()
	commandArg = execCmd.Arg("command line", "command to be executed").Required().String() // commandArg 要执行的命令
)

//// sftp
//var (
//	Sftp     = kingpin.Command("sftp", "call the sftp. (make sure \"sftp\" is available in your local.)").Alias("f") //SFTP 启动sftp
//	SftpArgs = Sftp.Arg("args", "sftp args").String()                                                                // 额外的参数
//)

// shellCmd shell 子命令
var (
	shellCmd = kingpin.Command("shell", "Run remote shell based on ssh protocol.").Alias("sh") // shellCmd shell子命令,用于模拟远程shell环境
)

var (
	version = kingpin.Command("version", "show the version.").Alias("v")
	v       = "0.3"
)

// Run 执行 SSH 客户端应用
func Run() {
	kingpin.Version(v)
	switch kingpin.Parse() {
	case execCmd.FullCommand():
		{
			runExec()
		}
	case shellCmd.FullCommand():
		{
			runShell()
		}
	case version.FullCommand():
		{
			runVersion()
		}
	}

}

func initConfig() (*gossh.Config, error) {
	config := &gossh.Config{
		Rand:              nil,
		RekeyThreshold:    0,
		KeyExchanges:      nil,
		Ciphers:           nil,
		MACs:              nil,
		User:              "",
		Auth:              []gossh.AuthMethod{},
		HostKeyCallback:   nil,
		BannerCallback:    nil,
		ClientVersion:     "",
		HostKeyAlgorithms: nil,
		Timeout:           0,
	}

	//if randFlag != nil {
	//	config.Rand = *randFlag
	//}

	if keyExchangeFlags != nil {
		config.KeyExchanges = *keyExchangeFlags
	}

	if cipherFlags != nil {
		config.Ciphers = *cipherFlags
	}

	if macFlags != nil {
		config.MACs = *macFlags
	}

	config.User = *userFlag
	config.Timeout = *timeoutFlag

	if ignoreKnownHostsFlag != nil && *ignoreKnownHostsFlag == true {
		config.HostKeyCallback = gossh.IgnoreHostKey
	} else {
		config.HostKeyCallback = gossh.NewKnownHostCallback(true, *knownHostsFlag)
	}

	if displayBannerFlag != nil && *displayBannerFlag == true {
		config.BannerCallback = gossh.DisplayBanner
	}

	if randFlag != nil {
		config.Rand = *randFlag
	}

	if forcePasswdFlag != nil && *forcePasswdFlag {
		method, err := gossh.ReadPasswordAuth(fmt.Sprintf("password for %s@%s:", *userFlag, *hostFlag))
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, method)
		return config, nil
	}

	if useAgentFlag != nil && *useAgentFlag == true {
		method, err := gossh.SSHAgentAuth()
		if err == nil {
			config.Auth = append(config.Auth, method)
		}
	}
	method, err := gossh.AuthByPrivateKeysFromPaths(*priKeyFlag)
	if err != nil {
		return nil, err
	}
	config.Auth = append(config.Auth, method)

	return config, nil
}

// runShell 启动shell模块的实现
func runShell() {
	config, err := initConfig()
	client, err := gossh.Connect(net.JoinHostPort(*hostFlag, *portFlag), config)
	if err != nil {
		fmt.Printf("An error occurred: %s\r\n", err)
		return
	}

	session, err := client.OpenSession()
	if err != nil {
		fmt.Printf("Open session failed: %s\r\n", err)
		return
	}

	err = session.PreparePty(*termFlag)
	if err != nil {
		fmt.Printf("Request pty failed: %s\r\n", err)
		return
	}

	if *keepAliveFlag {
		cancelKeepAlive := session.KeepAlive(*keepAliveIntervalFlag, 0)
		defer cancelKeepAlive()
	}

	if envsFlag != nil {
		session.SetEnvs(envsFlag)
	}

	var cancelUpdateWin context.CancelFunc = nil
	var updateErr error = nil
	if runtime.GOOS == "windows" {
		cancelUpdateWin, updateErr = session.AutoUpdateTerminalSizeForWindowsOS(time.Second)
		defer cancelUpdateWin()
	} else {
		cancelUpdateWin, updateErr = session.AutoUpdateTerminalSize()
		defer cancelUpdateWin()
	}
	if updateErr != nil {
		fmt.Printf("Updating windows sized failed\r\n")
	}

	cancelRow, err := session.MakeRow(os.Stdin)
	defer cancelRow()

	err = session.RedirectOutput(os.Stdout, os.Stderr)
	err = session.RedirectInput(os.Stdin)
	if err != nil {
		fmt.Printf("IO error: %s\r\n", err)
		return
	}

	err = session.Shell()
	if err != nil {
		fmt.Printf("Exit with error: %s\r\n", err)
	}
	fmt.Printf("Exit status %d\r\n", 0)
}

// runExec 远程命令执行模块的实现
func runExec() {
	config, err := initConfig()
	client, err := gossh.Connect(net.JoinHostPort(*hostFlag, *portFlag), config)
	if err != nil {
		fmt.Printf("An error occurred: %s\r\n", err)
		return
	}

	session, err := client.OpenSession()
	if err != nil {
		fmt.Printf("Open session failed: %s\r\n", err)
		return
	}

	if envsFlag != nil {
		session.SetEnvs(envsFlag)
	}

	if *ptyFlag {
		err = session.PreparePty(*termFlag)
		if err != nil {
			fmt.Printf("Request pty failed: %s\r\n", err)
			return
		}
		if runtime.GOOS == "windows" {
			cancelUpdateWin, updateErr := session.AutoUpdateTerminalSizeForWindowsOS(time.Second)
			if updateErr != nil {
				fmt.Printf("Updating windows sized failed\r\n")
			}
			defer cancelUpdateWin()
		} else {
			cancelUpdateWin, updateErr := session.AutoUpdateTerminalSize()
			if updateErr != nil {
				fmt.Printf("Updating windows sized failed\r\n")
			}
			defer cancelUpdateWin()
		}
		cancelRow, _ := session.MakeRow(os.Stdin)
		defer cancelRow()
	}

	if *keepAliveFlag {
		cancelKeepAlive := session.KeepAlive(*keepAliveIntervalFlag, 0)
		defer cancelKeepAlive()
	}

	err = session.RedirectOutput(os.Stdout, os.Stderr)
	err = session.RedirectInput(os.Stdin)
	if err != nil {
		fmt.Printf("IO error: %s\r\n", err)
		return
	}
	err = session.Exec(*commandArg)
	if err != nil {
		fmt.Printf("Exit with error: %s\r\n", err)
	}
	fmt.Printf("Exit status %d\r\n", 0)
}

// runVersion 打印版本信息
func runVersion() {
	fmt.Printf("version: %s\tAuthor: NiShoushun\tGithub: https://github.com/nishoushun/gossh\n\r", v)
}
