package input

import (
	"bufio"
	"os"
	"regexp"
	"unicode"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type WordlistInput struct {
	active   bool
	config   *ffuf.Config
	data     [][]byte
	position int
	keyword  string
}

func NewWordlistInput(keyword string, value string, conf *ffuf.Config) (*WordlistInput, error) {
	var wl WordlistInput
	wl.active = true
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

// Position will return the current position in the input list
func (w *WordlistInput) Position() int {
	return w.position
}

// SetPosition sets the current position of the inputprovider
func (w *WordlistInput) SetPosition(pos int) {
	w.position = pos
}

// ResetPosition resets the position back to beginning of the wordlist.
func (w *WordlistInput) ResetPosition() {
	w.position = 0
}

// Keyword returns the keyword assigned to this InternalInputProvider
func (w *WordlistInput) Keyword() string {
	return w.keyword
}

// Next will return a boolean telling if there's words left in the list
func (w *WordlistInput) Next() bool {
	return w.position < len(w.data)
}

// IncrementPosition will increment the current position in the inputprovider data slice
func (w *WordlistInput) IncrementPosition() {
	w.position += 1
}

// Value returns the value from wordlist at current cursor position
func (w *WordlistInput) Value() []byte {
	return w.data[w.position]
}

// Total returns the size of wordlist
func (w *WordlistInput) Total() int {
	return len(w.data)
}

// Active returns boolean if the inputprovider is active
func (w *WordlistInput) Active() bool {
	return w.active
}

// Enable sets the inputprovider as active
func (w *WordlistInput) Enable() {
	w.active = true
}

// Disable disables the inputprovider
func (w *WordlistInput) Disable() {
	w.active = false
}

// validFile checks that the wordlist file exists and can be read
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

// readFile reads the file line by line to a byte slice
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
	var ok bool
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
				text := reader.Text()

				if w.config.IgnoreWordlistComments {
					text, ok = stripComments(text)
					if !ok {
						continue
					}
				}
				data = append(data, []byte(text))
			}
		} else {
			text := reader.Text()

			if w.config.IgnoreWordlistComments {
				text, ok = stripComments(text)
				if !ok {
					continue
				}
			}

			// Check if line should be excluded based on filter options
			if shouldExcludeLine(text, w.config) {
				continue
			}

			data = append(data, []byte(text))
			if w.keyword == "FUZZ" && len(w.config.Extensions) > 0 {
				for _, ext := range w.config.Extensions {
					data = append(data, []byte(text+ext))
				}
			}
		}
	}
	w.data = data
	return reader.Err()
}

// stripComments removes all kind of comments from the word
func stripComments(text string) (string, bool) {
	// If the line starts with a # ignoring any space on the left,
	// return blank.
	if strings.HasPrefix(strings.TrimLeft(text, " "), "#") {
		return "", false
	}

	// If the line has # later after a space, that's a comment.
	// Only send the word upto space to the routine.
	index := strings.Index(text, " #")
	if index == -1 {
		return text, true
	}
	return text[:index], true
}

// shouldExcludeLine checks if a line should be excluded based on the filter options
func shouldExcludeLine(text string, conf *ffuf.Config) bool {
	trimmedText := strings.TrimSpace(text)

	// Skip empty lines
	if len(trimmedText) == 0 {
		return true
	}

	// -xc-c: Exclude lines starting with #, ~, or /
	if conf.ExcludeCommentLines {
		if strings.HasPrefix(trimmedText, "#") || 
		   strings.HasPrefix(trimmedText, "~") || 
		   strings.HasPrefix(trimmedText, "/") {
			return true
		}
	}

	// -xc-d: Exclude lines starting with .
	if conf.ExcludeDotLines {
		if strings.HasPrefix(trimmedText, ".") {
			return true
		}
	}

	// -xc-n: Exclude lines starting with numbers
	if conf.ExcludeNumberLines {
		if len(trimmedText) > 0 {
			firstChar := trimmedText[0]
			if firstChar >= '0' && firstChar <= '9' {
				return true
			}
		}
	}

	// -xc-upper: Exclude lines that are entirely uppercase
	if conf.ExcludeUppercase {
		isUpper := true
		for _, r := range trimmedText {
			if unicode.IsLetter(r) && !unicode.IsUpper(r) {
				isUpper = false
				break
			}
		}
		if isUpper && len(trimmedText) > 0 {
			return true
		}
	}

	// -xc-lower: Exclude lines that are entirely lowercase
	if conf.ExcludeLowercase {
		isLower := true
		for _, r := range trimmedText {
			if unicode.IsLetter(r) && !unicode.IsLower(r) {
				isLower = false
				break
			}
		}
		if isLower && len(trimmedText) > 0 {
			return true
		}
	}

	// -xc-s-upper: Exclude lines starting with uppercase letter
	if conf.ExcludeStartUpper {
		if len(trimmedText) > 0 {
			firstRune := rune(trimmedText[0])
			if unicode.IsUpper(firstRune) {
				return true
			}
		}
	}

	// -xc-s-lower: Exclude lines starting with lowercase letter
	if conf.ExcludeStartLower {
		if len(trimmedText) > 0 {
			firstRune := rune(trimmedText[0])
			if unicode.IsLower(firstRune) {
				return true
			}
		}
	}

	return false
}
