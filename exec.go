package conex

import (
	"bytes"
	"errors"
	"io"
)

// Cmd represents an external command being prepared or run.
// It has similar fields and methods to os/exec.Cmd.
type Cmd struct {
	// Path is the path of the command to run.
	Path string

	// Args holds command line arguments, including the command as Args[0].
	Args []string

	// Env specifies the environment of the process.
	Env []string

	// Dir specifies the working directory of the command.
	Dir string

	// Stdin specifies the process's standard input.
	Stdin io.Reader

	// Stdout and Stderr specify the process's standard output and error.
	Stdout io.Writer
	Stderr io.Writer

	start func() error
	wait  func() error
}

// Run starts the specified command and waits for it to complete.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Start starts the specified command but does not wait for it to complete.
func (c *Cmd) Start() error {
	if c.start != nil {
		return c.start()
	}
	return errors.New("exec: Start not implemented")
}

// Wait waits for the command to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
func (c *Cmd) Wait() error {
	if c.wait != nil {
		return c.wait()
	}
	return errors.New("exec: Wait not implemented")
}

// Output runs the command and returns its standard output.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("conex: Stdout already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	err := c.Run()
	return b.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("conex: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("conex: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}
