# Make SSH Client Easier With GO

本包对 `golang.org/x/crypto/ssh` 中的客户端相关类型进行了简单的包装，一般的使用者能够通过较少的代码量快速实现一个可正常使用的 SSH 客户端应用程序。

目前只提供了部分功能，后续会补上一些常用的 subsystem 的工具实现，例如 `sftp` 等。

### 安装

```bash
go get github.com/nishoushun/gossh
go get github.com/nishoushun/gossh/cli
```

### 示例

#### shell

连接一个 Shell 只需要以下几步：

1. 创建一个 `Config` 实例；
2. 调用 `Connect` 函数建立 SSH 连接，并获取一个 `SSHClient` 实例；
3. 用获取的 `SSHClient` 实例，打开一个新的会话；
4. 禁用标准输入的回显以及行缓；
5. 发送一个 `pty-req` 请求，附带上你的终端色彩模式；
6. 绑定会话的输入输出至本地；
7. 请求 `shell`，直至 `exit-status` 被接收；

> 以下示例以 Open-SSH 作为服务端.

**示例 1**：连接至目标机器的 shell

```go
package main

import (
	"fmt"
	"github.com/nishoushun/gossh"
	"log"
	"os"
)

func main() {
	client, err := gossh.Connect(":22", gossh.DefaultConfigAuthByPasswd("niss", "123456"))
	if err != nil {
		log.Fatalln(err)
	}
	sess, err := client.OpenSession()
	cancelRow, _ := sess.MakeRow(os.Stdin)
	defer cancelRow()
	cancelUpdate, _ := sess.AutoUpdateTerminalSize()
	defer cancelUpdate()
	err = sess.PreparePty("xterm-256color")
	if err != nil {
		log.Fatalln(err)
	}
	sess.RedirectToSTD()
	err = sess.Shell()
	if err != nil {
		fmt.Println(err)
	}
}
```

编译并运行即可进入 SSH Shell：

![image-20220501210651432](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012106482.png)

#### 远程命令执行

相对于 shell 来说，远程命令执行要简单的多。打开一个新的会话后，只需要执行 `RunForCombineOutput` 即可等待命令执行完毕，并返回执行结果。

**示例 2**：执行远程命令 `ls / -al`

```go
package main

import (
	"fmt"
	"github.com/nishoushun/gossh"
	"log"
)

func main() {
	client, err := gossh.Connect(":22", gossh.DefaultConfigAuthByPasswd("niss", "162333"))
	if err != nil {
		log.Fatalln(err)
	}
	sess, err := client.OpenSession()
	output, err := sess.RunForCombineOutput("ls / -al")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(output))
}
```

![image-20220501211528455](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012115527.png)

另外本包也支持像执行 shell 一样执行一个交互式远程命令。

**示例 3**：执行 `htop` 类型的交互式命令

```go
package main

import (
	"fmt"
	"github.com/nishoushun/gossh"
	"log"
	"os"
)

func main() {
	client, err := gossh.Connect(":22", gossh.DefaultConfigAuthByPasswd("niss", "162333"))
	if err != nil {
		log.Fatalln(err)
	}
	sess, err := client.OpenSession()
	cancelRow, _ := sess.MakeRow(os.Stdin)
	defer cancelRow()
	cancelUpdate, _ := sess.AutoUpdateTerminalSize()
	defer cancelUpdate()
	sess.RedirectToSTD()
	err = sess.RunWithPty("htop", "xterm-256color")
	if err != nil {
		fmt.Println(err)
	}
}
```

![image-20220501212206859](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012122949.png)

#### 端口转发

对于端口转发功能，原有的 ssh 包也提供了相应的 `Dial` 与 `Listen` 来实现相应的功能。但不同于命令执行，这需要写代码的人员了解 SSH 协议的 `direct-tcpip` 以及 `tcpip-forward` 类型的 ssh channel 的机制。

`Dial` 通过已经建立 ssh 的连接，请求一个新的信道来连接至远程端口，并返回一个 `net.Conn` 接口的实现，本质上是 `direct-tcpip` 请求；

`Listen` 通过已经建立 ssh 的连接，让服务端监听一个端口，并且其连接都将转发至本地客户端，因此 `Listen` 会返回一个 `net.Listener` 来接受转发的网络连接。

也就是说，原有 `ssh` 包已经实现了两种转发功能，而且上述两种方法也同样被包装进 `SSHClient` 类型中。

#### 客户端 Demo

`cli` 包下面实现了一个基础的客户端 Demo，本 Demo 只实现了一个简单的 shell 以及 命令执行请求，后续会补上 `SSHClient` 的 sftp 功能。

至于 `direct-tcp` （Open-SSH 的 `ssh -L` ）以及 `tcpip-forward` （Open-SSH 的 `ssh -R`） 功能，聪明的你一定能用 `SSHClient` 提供的 # `Dial` 与 `Listen` 实现！😎

![image-20220327032755721](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037229.png)



##### 编译 & 运行

`cli` 包下包含了一个 `Run` 方函数，用于执行一个很简单的 SSH 客户端 demo，只需要导入包，编译后就可运行：

```go
package main

import "github.com/nishoushun/gossh/cli"

func main() {
	cli.Run()
}
```

##### Usage

```go
➜  ssh ./ssh --help             
usage: ssh [<flags>] <command> [<args> ...]

Flags:
      --help                     Show context-sensitive help (also try --help-long and --help-man).
  -p, --port="22"                port of ssh host.
  -h, --host=HOST                the host address of ssh host, ip address or domain name.
  -u, --user="niss"              specified user to login.
  -e, --env=ENV ...              set environment.
      --display-banner           display server banner.
  -k, --private-key="/home/niss/.ssh/id_rsa"  
                                 use specified private key file.
  -a, --ssh-agent                use ssh-agent for authentication.
  -P, --passwd                   force to use password.
      --known-hosts="/home/niss/.ssh/known_hosts"  
                                 use specified known hosts file.
  -t, --timeout=0s               timeout for connection.
  -z, --term="xterm-256color"    use the given terminal-color mod to run the interactive command line or shell.
  -l, --keep-alive               set the interval of keepalive request.
  -i, --keep-alive-interval=60s  set the interval of keepalive request.
      --ignore-host-key          do not check the server's host key.
      --cipher=CIPHER ...        choose cipher algorithm
      --key-exchange=KEY-EXCHANGE ...  
                                 choose key exchange algorithm
      --mac=MAC ...              choose message authentication code algorithm
      --rand=RAND                choose a src to read random bytes
      --version                  Show application version.

Commands:
  help [<command>...]
    Show help.

  exec [<flags>] <command line>
    execute command.

  shell
    Run remote shell based on ssh protocol.

  version
    show the version.

```

##### 创建ssh终端

![Peek 2022-03-27 03-32](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037363.gif)

##### 添加新主机至 `known_hosts`

![Peek 2022-03-27 03-35](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037246.gif)

##### 命令执行

![Peek 2022-03-27 03-37](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037354.gif)

##### 交互式执行命令

本客户端可以获取伪终端，并能检测本地终端大小的变化，同步到远程终端：

![Peek 2022-03-27 03-45](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037590.gif)

##### 身份验证

* `-P`：使用密码验证
* `-a, --ssh-agent`：使用ssh-agent验证
* `-k, --private-key`：私钥文件路径（默认为 `～/.ssh/id_rsa`）
* `--known-hosts`：known_hosts 文件路径（默认为 `～/.ssh/known_hosts `）

##### 密码算法组件选项

* `--cipher`：选取加密算法

  **支持：**`aes128-ct`、`aes192-ctr`、`aes256-ctr`、`aes128-gcm@openssh.com`、`chacha20-poly1305@openssh.com`、`arcfour256`、`arcfour128`、`arcfour`、`aes128-cbc`、`3des-cbc`

* `--key-exchange`：选取密钥交换算法

  **支持**：`curve25519-sha256`、`curve25519-sha256@libssh.org`、`ecdh-sha2-nistp256`、`ecdh-sha2-nistp384`、`ecdh-sha2-nistp521`、`diffie-hellman-group14-sha256`、`diffie-hellman-group14-sha1`、`diffie-hellman-group1-sha1`

* `--mac`：选取消息摘要算法

  **支持：**`ssh-rsa`、`rsa-sha2-256`、`rsa-sha2-512`、`ssh-dss`、`ecdsa-sha2-nistp256`、`ecdsa-sha2-nistp384`、`ecdsa-sha2-nistp521`、`sk-ecdsa-sha2-nistp256@openssh.com`、`sk-ssh-ed25519@openssh.com`

  > 该系列选项可以通过多次指定以添加多个算法，例如：
  >
  > ```bash
  > gossh connect --cipher aes128-ctr --cipher aes256-ctr
  > ```

* `--rand`：要使用的随机数源，在linux系统中默认为 `/dev/urandom`

