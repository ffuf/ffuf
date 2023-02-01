package ffuf

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ConfigOptionsHistory struct {
	ConfigOptions
	Time time.Time `json:"time"`
}

func WriteHistoryEntry(conf *Config) (string, error) {
	options := ConfigOptionsHistory{
		ConfigOptions: conf.ToOptions(),
		Time:          time.Now(),
	}
	jsonoptions, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	hashstr := calculateHistoryHash(jsonoptions)
	err = createConfigDir(filepath.Join(HISTORYDIR, hashstr))
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(HISTORYDIR, hashstr, "options"), jsonoptions, 0640)
	return hashstr, err
}

func calculateHistoryHash(options []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(options))
}
