package input

import (
	"bufio"
	"os"
	"regexp"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type WordlistInput struct {
	config   *ffuf.Config
	data     [][]byte
	position int
	keyword  string
}

func NewWordlistInput(keyword string, value string, conf *ffuf.Config) (*WordlistInput, error) {
	var wl WordlistInput
	wl.keyword = keyword
	wl.config = conf
	wl.position = 0
	var valid bool
	var err error
	// stdin?
	if value == "-" {
		// yes
		valid = true
	} else {
		// no
		valid, err = wl.validFile(value)
	}
	if err != nil {
		return &wl, err
	}
	if valid {
		err = wl.readFile(value)
	}
	return &wl, err
}

//Position will return the current position in the input list
func (w *WordlistInput) Position() int {
	return w.position
}

//ResetPosition resets the position back to beginning of the wordlist.
func (w *WordlistInput) ResetPosition() {
	w.position = 0
}

//Keyword returns the keyword assigned to this InternalInputProvider
func (w *WordlistInput) Keyword() string {
	return w.keyword
}

//Next will increment the cursor position, and return a boolean telling if there's words left in the list
func (w *WordlistInput) Next() bool {
	if w.position >= len(w.data) {
		return false
	}
	return true
}

//IncrementPosition will increment the current position in the inputprovider data slice
func (w *WordlistInput) IncrementPosition() {
	w.position += 1
}

//Value returns the value from wordlist at current cursor position
func (w *WordlistInput) Value() []byte {
	return w.data[w.position]
}

//Total returns the size of wordlist
func (w *WordlistInput) Total() int {
	return len(w.data)
}

//validFile checks that the wordlist file exists and can be read
func (w *WordlistInput) validFile(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	f.Close()
	return true, nil
}

//readFile reads the file line by line to a byte slice
func (w *WordlistInput) readFile(path string) error {
	var file *os.File
	var err error
	if path == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(path)
		if err != nil {
			return err
		}
	}
	defer file.Close()

	var data [][]byte
	reader := bufio.NewScanner(file)
	re := regexp.MustCompile(`(?i)%ext%`)
	for reader.Scan() {
		if w.config.DirSearchCompat && len(w.config.Extensions) > 0 {
			text := []byte(reader.Text())
			if re.Match(text) {
				for _, ext := range w.config.Extensions {
					contnt := re.ReplaceAll(text, []byte(ext))
					data = append(data, []byte(contnt))
				}
			} else {
				data = append(data, []byte(reader.Text()))
			}
		} else {
			data = append(data, []byte(reader.Text()))
			if w.keyword == "FUZZ" && len(w.config.Extensions) > 0 {
				for _, ext := range w.config.Extensions {
					data = append(data, []byte(reader.Text()+ext))
				}
			}
		}
	}
	w.data = data
	return reader.Err()
}
