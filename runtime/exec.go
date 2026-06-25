package runtime

import (
	"bytes"
	"errors"
	"io"
	"sync"
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

	mu      sync.Mutex
	start   func() error
	wait    func() error
	started bool
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Stdout == nil {
		return errors.New("exec: Stdout must be set to an io.Writer")
	}
	if c.Stderr == nil {
		return errors.New("exec: Stderr must be set to an io.Writer")
	}
	if c.started {
		return errors.New("exec: already started")
	}
	c.started = true
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
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil, errors.New("exec: already started")
	}
	var b bytes.Buffer
	c.Stdout = &b
	if c.Stderr == nil {
		c.Stderr = io.Discard
	}
	c.mu.Unlock()
	err := c.Run()
	return b.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil, errors.New("exec: already started")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	c.mu.Unlock()
	err := c.Run()
	return b.Bytes(), err
}

// WireCommand is for internal use.
func WireCommand(c *Cmd, start func() error, wait func() error) *Cmd {
	c.start = start
	c.wait = wait
	return c
}
