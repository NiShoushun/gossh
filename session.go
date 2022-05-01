package gossh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Session 对 ssh.Session 的一个包装，提供了 session 通道的处理工具函数。
// 对于同一个 Session 实例，任何命令或 shell 的只能执行1次，当收到服务器的 exit-status 消息时，底层的通道将被关闭。
// 对于绝大部分 SSH 服务器，shell、命令执行都不存在问题，但是环境变量以及信号的请求可能会限制于服务器的具体实现。
type Session struct {
	sess *ssh.Session
	sync.Mutex
}

func (s *Session) Close() error {
	return s.sess.Close()
}

// PreparePty 发送一个 pty-req 请求，附带的窗口大小信息从当前的标准输出文件中获取。
// termMode 为 终端色彩模式
func (s *Session) PreparePty(termMode string) error {
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}

	return s.PreparePtyWithSize(termMode, termWidth, termHeight)
}

// PreparePtyWithSize 发送一个指定窗口大小的 pty-req 请求。
func (s *Session) PreparePtyWithSize(termMode string, termWidth, termHeight int) error {
	if err := s.sess.RequestPty(termMode, termWidth, termHeight, ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return err
	}
	return nil
}

// Shell 发送一个 shell 请求，并阻塞至 exit-status 消息被接收
func (s *Session) Shell() error {
	if err := s.sess.Shell(); err != nil {
		return err
	}
	return s.sess.Wait()
}

// Exec Shell 发送一个 exec 请求，并阻塞至 exit-status 消息被接收
func (s *Session) Exec(cmdline string) error {
	return s.sess.Run(cmdline)
}

// RunWithPty 先发送 pty-req 请求（窗口大小为当前终端的窗口大小），之后会发送一个 exec 请求。
// 该函数将会阻塞直至 exit-status 被接收或 IO 出现错误。
func (s *Session) RunWithPty(command, termMode string) error {
	if err := s.PreparePty(termMode); err != nil {
		return err
	}
	return s.Exec(command)
}

// KeepAlive 以 interval 为间隔发送请求，保持连接不被中断，这取决于服务器的网络设计以及 SSH 服务请求处理的实现。
// 当没有成功受到发送的请求的回应次数达到 maxErrRetries 时，将会停止并退出。
// maxErrRetries 小于等于 0 时将会一直发送，直至返回函数被调用或出现IO错误。
func (s *Session) KeepAlive(interval time.Duration, maxErrRetries int) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	if interval <= 0 {
		return cancel
	}

	if maxErrRetries <= 0 {
		go func() {
			// 产生一个定时通知器
			ticker := time.NewTicker(interval)
			for {
				select {
				// 等待取消任务通知
				case <-ctx.Done():
					return
				// 等待计时器通知
				case <-ticker.C:
					// 发送一个请求
					s.sess.SendRequest("keepalive@nss", true, nil)
				}
			}
		}()
		return cancel
	}

	go func() {
		// 产生一个定时通知器
		ticker := time.NewTicker(interval)
		for {
			select {
			// 等待取消任务通知
			case <-ctx.Done():
				return
			// 等待计时器通知
			case <-ticker.C:
				// 发送一个请求
				_, err := s.sess.SendRequest("keepalive@nss", true, nil)
				if err != nil {
					if maxErrRetries <= 0 {
						return
					}
					maxErrRetries -= 1
				}
			}
		}
	}()

	return cancel

}

// RedirectToSTD 重定向 session 的输入、输出至当前程序的标准输入、输出。
func (s *Session) RedirectToSTD() error {
	if err := s.RedirectOutput(os.Stdout, os.Stderr); err != nil {
		return err
	}
	if err := s.RedirectInput(os.Stdin); err != nil {
		return err
	}
	return nil
}

// RedirectOutput 重定向 session 的输出，返回的 error 不为 nil 表明无法正确地从 session 中获取输出流。
func (s *Session) RedirectOutput(out, err io.Writer) error {
	sessionOut, e := s.sess.StdoutPipe()
	if e != nil {
		return e
	}
	sessionErr, e := s.sess.StderrPipe()
	if e != nil {
		return e
	}
	go io.Copy(out, sessionOut)
	go io.Copy(err, sessionErr)
	return nil
}

// RedirectInput 重定向 in 至 session 的输入，返回的 error 不为 nil 表明无法正确地从 session 中获取输入流。
func (s *Session) RedirectInput(in io.Reader) error {
	sessionIn, err := s.sess.StdinPipe()
	if err != nil {
		return err
	}
	go io.Copy(sessionIn, in)
	return nil
}

// SendWinChange 从fd中获取终端大小并与传入参数比较，如果发生变化则向该会话发送一个 window-change 请求。
// 返回最新检测的窗口宽度与长度
func (s *Session) SendWinChange(fd, oldTermHeight, oldTermWidth int) (int, int, error) {
	newTermWidth, newTermHeight, err := term.GetSize(fd)
	if err != nil {
		return oldTermWidth, oldTermHeight, err
	}
	if newTermHeight == oldTermHeight && newTermWidth == oldTermWidth {
		return newTermWidth, newTermHeight, nil
	}
	err = s.sess.WindowChange(newTermHeight, newTermWidth)
	if err != nil {
		return oldTermWidth, oldTermHeight, err
	}
	return newTermWidth, newTermHeight, nil
}

// AutoUpdateTerminalSize 自动检测窗口变化，并发送给服务端实现远程终端大小的同步。
// 使用返回的 CancelFunc 取消同步。
// 如果返回的 error 不为 nil，表明无法获取终端大小。
// 注意：此函数并不适用于 windows 系统，请用 AutoUpdateTerminalSizeForWindowsOS。
func (s *Session) AutoUpdateTerminalSize() (context.CancelFunc, error) {
	// 从本终端的标准输出获取文件描述符
	fd := int(os.Stdout.Fd())
	// 先发送一个windowsChange消息，同步双方终端窗口大小
	termWidth, termHeight, err := s.SendWinChange(fd, 0, 0)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	// 对于其他系统，例如 unix系统，通过捕获信号量的方式来得知终端大小发生变化;
	// 在 windows 系统中没有定义该信号量 syscall.SIGWINCH （0x1c）( 具体定义： https://pkg.go.dev/syscall#pkg-constants),
	// 由于Goland windows 平台没有定义该常量，故直接使用整数
	var sigwinch syscall.Signal = 0x1c
	// 捕获系统的终端窗口变化信号
	sigwinchChan := make(chan os.Signal, 1)
	// 由程序捕获终端窗口变化信号量
	signal.Notify(sigwinchChan, sigwinch)
	// 开启一个协程去检测终端大小变化，并向会话发送终端变化请求
	go func() {
		for {
			select {
			// 阻塞读取窗口变化信号量
			case <-sigwinchChan:
				{
					termWidth, termHeight, err = s.SendWinChange(fd, termHeight, termWidth)
				}
			// 接收到取消更新窗口大小信号，直接退出协程
			case <-ctx.Done():
				{
					return
				}
			}
		}
	}()
	return cancel, nil
}

// AutoUpdateTerminalSizeForWindowsOS 适用于 Windows 系统的窗口变化自动检测并同步。
// interval 为两次检测之间的时间间隔
func (s *Session) AutoUpdateTerminalSizeForWindowsOS(interval time.Duration) (context.CancelFunc, error) {
	// 从本终端的标准输出获取文件描述符
	fd := int(os.Stdout.Fd())
	// 先发送一个windowsChange消息，同步双方终端窗口大小
	termWidth, termHeight, err := s.SendWinChange(fd, 0, 0)
	if err != nil {
		return nil, err
	}

	// 如果使用 windows 系统，通过定时查询方式获取窗口大小，以此来判断窗口是否改变
	if "windows" == runtime.GOOS {
		ctx, cancel := context.WithCancel(context.Background())
		// 初始ticker发送信号时间间隔很长是为了防止在unix系统中定时更新窗口大小
		ticker := time.Tick(interval)
		// 开启一个协程去检测终端大小变化，并向会话发送终端变化请求
		go func() {
			for {
				select {
				// 计时器信号
				case <-ticker:
					{
						termWidth, termHeight, err = s.SendWinChange(fd, termHeight, termWidth)
					}
				// 接收到取消更新窗口大小信号，直接退出协程
				case <-ctx.Done():
					{
						return
					}
				}
			}
		}()
		return cancel, nil
	}
	return nil, errors.New("not windows OS")
}

// MakeRow makeRow 禁用输入缓存，若禁用成功，返回恢复函数；否则返回 error 不为 nil
// 如果恢复函数执行结果为 error 不为 nil,则表明无法恢复文件的原有模式
func (s *Session) MakeRow(input *os.File) (func() error, error) {
	// fixme 在Linux系统中在关闭输入缓存的情况下，debug级别日志会显示错位，故要关闭 make raw
	// windows正常，其它系统未测试
	//client.logger.Warnf("you are using '%s\n'", runtime.GOOS)
	//if runtime.GOOS == "linux" {
	//	if client.logger.GetLevel() == log.DebugLevel {
	//		client.logger.Warnln("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	//		client.logger.Warnln("@ If you are using raw input in unix system and set log in debug level or more detailed , @")
	//		client.logger.Warnln("@ that kind of messages would display in wrong place.                                     @")
	//		client.logger.Warnln("@ So I didn't make the input in raw mod, it maybe cause some input problems.              @")
	//		client.logger.Warnln("@                                                               >-< written by NiShoushun @")
	//		client.logger.Warnln("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	//		return func() {}, nil
	//	}
	//}

	// 尝试禁止使用输入缓存，将键盘输入直接输入到程服务器
	fd := int(input.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	return func() error {
		return term.Restore(fd, state)
	}, nil
}

// SetEnvs 为session设置环境变量，遍历 envs 并发送 env 请求；是否起作用取决于服务器的实现。
// 返回最后一个发生的错误；
func (s *Session) SetEnvs(session *ssh.Session, envs *map[string]string) error {
	if envs != nil {
		var err error = nil
		for k, v := range *envs {
			e := session.Setenv(k, v)
			if e != nil {
				err = e
			}
		}
		return err
	}
	return nil
}

// SetEnv 为当前会话设置环境变量；是否起作用取决于服务器的实现。
func (s *Session) SetEnv(name, value string) error {
	return s.sess.Setenv(name, value)
}

// RunForCombineOutput 执行命令并等待至结束，返回远程执行结果的全部的输出。
func (s *Session) RunForCombineOutput(command string) ([]byte, error) {
	return s.sess.CombinedOutput(command)
}

// RunForOutput 执行命令并等待至结束，返回远程执行结果的全部的标准输出。
func (s *Session) RunForOutput(command string) ([]byte, error) {
	return s.sess.Output(command)
}

// SendSignal 发送 signal 请求至服务端；是否起作用取决于服务器的实现。
func (s *Session) SendSignal(sig Signal) error {
	return s.sess.Signal(ssh.Signal(sig))
}

// Signal ssh 包导出的信号定义
type Signal string

const (
	SIGABRT Signal = "ABRT"
	SIGALRM Signal = "ALRM"
	SIGFPE  Signal = "FPE"
	SIGHUP  Signal = "HUP"
	SIGILL  Signal = "ILL"
	SIGINT  Signal = "INT"
	SIGKILL Signal = "KILL"
	SIGPIPE Signal = "PIPE"
	SIGQUIT Signal = "QUIT"
	SIGSEGV Signal = "SEGV"
	SIGTERM Signal = "TERM"
	SIGUSR1 Signal = "USR1"
	SIGUSR2 Signal = "USR2"
)
