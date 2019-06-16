package input

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type CommandInput struct {
	config *ffuf.Config
	count  int
}

func NewCommandInput(conf *ffuf.Config) (*CommandInput, error) {
	var cmd CommandInput
	cmd.config = conf
	cmd.count = -1
	return &cmd, nil
}

//Next will increment the cursor position, and return a boolean telling if there's iterations left
func (c *CommandInput) Next() bool {
	c.count++
	if c.count >= c.config.InputNum {
		return false
	}
	return true
}

//Value returns the input from command stdoutput
func (c *CommandInput) Value() []byte {
	var stdout bytes.Buffer
	os.Setenv("FFUF_NUM", strconv.Itoa(c.count))
	cmd := exec.Command(SHELL_CMD, SHELL_ARG, c.config.InputCommand)
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
