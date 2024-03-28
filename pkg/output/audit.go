package output

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
)

type AuditLogger struct {
	file *os.File
	lock sync.Mutex
}

func NewAuditLogger(filename string) (*AuditLogger, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	auditLogger := &AuditLogger{file: f}

	return auditLogger, nil
}

func (logger *AuditLogger) Close() {
	logger.file.Close()
}

func (logger *AuditLogger) Write(data interface{}) error {
	logger.lock.Lock()
	defer logger.lock.Unlock()

	d := struct {
		Type string
		Data interface{}
	}{
		reflect.TypeOf(data).String(),
		data,
	}

	j, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %s", err)
	}

	_, err = logger.file.Write(j)
	if err != nil {
		return fmt.Errorf("could not write json data to audit log: %s", err)
	}

	_, err = logger.file.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("could not write newline to underlying io.Writer: %w", err)
	}

	return nil
}
