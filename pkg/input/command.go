package input

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type CommandInput struct {
	config  *ffuf.Config
	count   int
	keyword string
	command string
}

func NewCommandInput(keyword string, value string, conf *ffuf.Config) (*CommandInput, error) {
	var cmd CommandInput
	cmd.keyword = keyword
	cmd.config = conf
	cmd.count = 0
	cmd.command = value
	return &cmd, nil
}

//Keyword returns the keyword assigned to this InternalInputProvider
func (c *CommandInput) Keyword() string {
	return c.keyword
}

//Position will return the current position in the input list
func (c *CommandInput) Position() int {
	return c.count
}

//ResetPosition will reset the current position of the InternalInputProvider
func (c *CommandInput) ResetPosition() {
	c.count = 0
}

//IncrementPosition increments the current position in the inputprovider
func (c *CommandInput) IncrementPosition() {
	c.count += 1
}

//Next will increment the cursor position, and return a boolean telling if there's iterations left
func (c *CommandInput) Next() bool {
	return c.count < c.config.InputNum
}

//Value returns the input from command stdoutput
func (c *CommandInput) Value() []byte {
	var stdout bytes.Buffer
	os.Setenv("FFUF_NUM", strconv.Itoa(c.count))
	cmd := exec.Command(SHELL_CMD, SHELL_ARG, c.command)
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return []byte("")
	}
	return stdout.Bytes()
}

//Total returns the size of wordlist
func (c *CommandInput) Total() int {
	return c.config.InputNum
}
