# Make SSH Client Easier With GO

本包对 `golang.org/x/crypto/ssh` 中的客户端相关类型进行了简单的包装，一般的使用者能够通过较少的代码量快速实现一个可正常使用的 SSH 客户端应用程序。

目前只提供了部分功能，后续会补上一些常用的 subsystem 的工具实现，例如 `sftp` 等。

### 安装

```bash
go get github.com/nishoushun/gossh
go get github.com/nishoushun/gossh/cli
```

### shell & exec

构建一个 `Config`，通过 Connect 函数获取一个 `SSHClient` 实例，然后使用。

#### shell

连接一个 Shell 只需要以下几步：

1. 创建一个 `Config` 实例；`Config` 类型包含了 `ssh.Config` 与 `ssh.ClientConfig` 的所有字段，用于配置客户端以及连接的选项；
2. 调用 `Connect` 函数建立 SSH 连接，并获取一个 `SSHClient` 实例；
3. 用获取的 `SSHClient` 实例，调用 `OpenSession` 打开一个新的会话；
4. `MakeRow` 函数禁用标准输入的回显以及行缓；
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
	client, err := gossh.Connect(":22", gossh.DefaultConfigAuthByPasswd("niss", "123456"))
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
	client, err := gossh.Connect(":22", gossh.DefaultConfigAuthByPasswd("niss", "123456"))
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

### 端口转发

对于端口转发功能，原有的 ssh 包也提供了相应的 `Dial` 与 `Listen` 来实现相应的功能。但不同于命令执行，这需要写代码的人员了解 SSH 协议的 `direct-tcpip` 以及 `tcpip-forward` 类型的 ssh channel 的机制。

`Dial` 通过已经建立 ssh 的连接，请求一个新的信道来连接至远程端口，并返回一个 `net.Conn` 接口的实现，本质上是 `direct-tcpip` 请求；

`Listen` 通过已经建立 ssh 的连接，让服务端监听一个端口，并且其连接都将转发至本地客户端，因此 `Listen` 会返回一个 `net.Listener` 来接受转发的网络连接。

也就是说，原有 `ssh` 包已经实现了两种转发功能，而且上述两种方法也同样被包装进 `SSHClient` 类型中。

#### Director

`Director` 提供了一些数据传送的函数，来简化端口转发的操作过程。其本质上是 `direct-tcpip` 信道的处理，即对 `Dial` 以及 `DialTcp` 函数的更高级封装。

通过 `SSHClient` 的 `NewDirector` 创建一个该类型实例。

**示例 4**：转发本地连接至远程服务器

由于没有远程服务器，先用 docker 凑合一下。没有设置端口映射，如果需要访问web服务，要通过 `172.17.0.2:80`

![image-20220502180620629](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205021806731.png)

访问本地地址无法访问 web 服务：

![image-20220502181005228](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205021810290.png)

使用 `172.17.0.2:80` 访问：

![image-20220502181027352](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205021810390.png)

下面代码展示了如何转发单个网络连接（TCP连接）的数据：

```go
package main

import (
	"fmt"
	"github.com/nishoushun/gossh"
	"log"
	"net"
	"os"
	"os/signal"
)

func main() { 
	client, err := gossh.Connect("127.0.0.1:22", gossh.DefaultConfigAuthByPasswd("niss", "123456"))
	if err != nil {
		log.Fatalln("ssh failed:", err)
		return
	}
	network := "tcp"
	listener, err := net.Listen(network, "127.0.0.1:80")
	if err != nil {
		log.Fatalln("listen failed:", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c, _ := listener.Accept()
	client.NewDirector().BindConnTo(c, "tcp", "172.17.0.2:80", ctx, ctx)
}
```

这里读写用了同一个 `context` 进行控制，实际上可以用两个 `context` 分别控制 `本地连接->远程服务端的连接` 以及 `远程服务端的连接->本地连接` 的数据传输。

编译并运行，访问 `127.0.0.1:80` 可成功转发该 TCP 连接至 `172.17.0.2:80`：

![image-20220502181620361](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205021816433.png)

> 其实也可以发现，虽然只转发一条 tcp 连接，但是可以 F5 刷新多次，是由于 HTTP 的 `Conntection: keep-alive` 复用了同一个 TCP 连接。
>
> 稍微修改一下包装一下传入的 `conn`，使其 `read` 函数将数据输出至标准输出流，可以看到浏览器发送的数据：
>
> ![image-20220502195459311](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205021954406.png)

**示例 4**：监听端口，并转发本地连接至远程服务器

```go
import (
	"fmt"
	"github.com/nishoushun/gossh"
	"golang.org/x/net/context"
	"log"
	"net"
	"os"
	"os/signal"
)

func main() {
	client, err := gossh.Connect("127.0.0.1:22", gossh.DefaultConfigAuthByPasswd("niss", "123456"))
	if err != nil {
		log.Fatalln("ssh failed:", err)
		return
	}
	network := "tcp"
	listener, err := net.Listen(network, "127.0.0.1:80")
	if err != nil {
		log.Fatalln("listen failed:", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	client.NewDirector().RedirectTo(listener, network, "172.17.0.2:80", ctx)
}
```

> **注意**：无论是 `cancelFunc` 的调用、 IO 错误，还是正常的传输完毕，都将导致两个网络连接被关闭。

##### NewConnCallback

另外 Director 也拥有一个 `NewConnCallback func(conn net.Conn) (net.Conn, error)` 类型的字段，在 `RedirectTo` 方法中当 `listener` 接收到一个新的连接，该函数将被调用，如果返回的 `error` 不为 `nil`，将终止本次转发过程。

该函数目的在于对一个新的连接进行记录、检查以及对 `net.Conn` 接口实例进行转换，以支持更多功能。

### 客户端 Demo

`cli` 包下面实现了一个基础的客户端 Demo，本 Demo 只实现了一个简单的 shell 以及 命令执行请求，后续可能会补上 `SSHClient` 的 sftp 以及一些其它功能。

![image-20220327032755721](https://ni187note-pics.oss-cn-hangzhou.aliyuncs.com/notes-img/202205012037229.png)

#### 编译 & 运行

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
  -l, --keep-alive               send useless request to keep tcp connection alive.
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

