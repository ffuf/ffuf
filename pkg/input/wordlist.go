package input

import (
	"bufio"
	"os"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type WordlistInput struct {
	config   *ffuf.Config
	data     [][]byte
	position int
}

func NewWordlistInput(conf *ffuf.Config) (*WordlistInput, error) {
	var wl WordlistInput
	wl.config = conf
	wl.position = -1
	valid, err := wl.validFile(conf.Wordlist)
	if err != nil {
		return &wl, err
	}
	if valid {
		err = wl.readFile(conf.Wordlist)
	}
	return &wl, err
}

func (w *WordlistInput) Next() bool {
	w.position++
	if w.position >= len(w.data)-1 {
		return false
	}
	return true
}

func (w *WordlistInput) Value() []byte {
	return w.data[w.position]
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
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var data [][]byte
	reader := bufio.NewScanner(file)
	for reader.Scan() {
		data = append(data, []byte(reader.Text()))
	}
	w.data = data
	return reader.Err()
}
