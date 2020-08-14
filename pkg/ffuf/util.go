package ffuf

import (
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

//used for random string generation in calibration function
var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

//RandomString returns a random string of length of parameter n
func RandomString(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

//UniqStringSlice returns an unordered slice of unique strings. The duplicates are dropped
func UniqStringSlice(inslice []string) []string {
	found := map[string]bool{}

	for _, v := range inslice {
		found[v] = true
	}
	ret := []string{}
	for k, _ := range found {
		ret = append(ret, k)
	}
	return ret
}

//FileExists checks if the filepath exists and is not a directory
func FileExists(path string) bool {
	md, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !md.IsDir()
}

//SizeofTTY return two integers or an error (X, Y, error) that corresspond to the width and height of the TTY and any error that could occur.
func SizeofTTY() (int, int, error) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	output := string(out)

	parts := strings.Split(output, " ")
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	y, err := strconv.Atoi(strings.Replace(parts[1], "\n", "", 1))
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

//Percent take current completed jobs and total jobs and return the percentage completed
func Percent(current int, total int) float32 {
	return ((float32(current) * float32(100)) / float32(total))
}
