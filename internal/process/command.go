//go:generate mockgen -source=command.go -destination=command_mock.go -package=process
package process

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
)

type Command interface {
	Start() error
	Wait() error
	Stop() error
	Kill() error
	ProcessState() *os.ProcessState
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	SetEnv([]string)
}

type DefaultCommand struct {
	cmd *exec.Cmd
}

type CommandExecutor interface {
	NewCommand(name string, arg ...string) Command
}

type DefaultCommandExecutor struct{}

func (d *DefaultCommandExecutor) NewCommand(name string, arg ...string) Command {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &DefaultCommand{cmd: cmd}
}

func (d *DefaultCommand) Start() error {
	// Allow to exec programs in the current directory
	if errors.Is(d.cmd.Err, exec.ErrDot) {
		d.cmd.Err = nil
	}
	return d.cmd.Start()
}

func (d *DefaultCommand) Wait() error {
	return d.cmd.Wait()
}

func (d *DefaultCommand) Stop() error {
	return d.cmd.Process.Signal(syscall.SIGTERM)
}

func (d *DefaultCommand) Kill() error {
	return d.cmd.Process.Kill()
}

func (d *DefaultCommand) ProcessState() *os.ProcessState {
	return d.cmd.ProcessState
}

func (d *DefaultCommand) SetStdout(w io.Writer) {
	d.cmd.Stdout = w
}

func (d *DefaultCommand) SetStderr(w io.Writer) {
	d.cmd.Stderr = w
}

func (d *DefaultCommand) SetEnv(env []string) {
	d.cmd.Env = env
}
