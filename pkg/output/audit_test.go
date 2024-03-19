package output

import (
	"os"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestAuditLogger(t *testing.T) {
	file, _ := os.CreateTemp("", "prefix")
	filename := file.Name()
	file.Close()
	audit, err := NewAuditLogger(filename)

	if err != nil {
		t.Errorf("Error creating audit logger: %s", err)
	}

	if audit == nil {
		t.Errorf("Audit was nil, expected non-nil")
	}

	defer audit.Close()

	os.Chmod(filename, 000)

	audit, err = NewAuditLogger(filename)

	if err == nil {
		t.Errorf("Error was nil, expected non-nil")
	}

	if audit != nil {
		t.Errorf("Audit was non-nil, expected nil")
	}

	os.Chmod(filename, 6550)
	os.Remove(filename)
}

func TestAuditLogWrite(t *testing.T) {
	expected := `{"Name":"ffuf.Config","Data":{"auditlog":"","autocalibration":false,"autocalibration_keyword":"","autocalibration_perhost":false,"autocalibration_strategies":null,"autocalibration_strings":null,"colors":false,"cmdline":"","configfile":"","postdata":"{\"quote\":\"I'll still be here tomorrow to high five you yesterday, my friend. Peace.\"}","debuglog":"","delay":{"Min":0,"Max":0,"IsRange":false,"HasDelay":false},"dirsearch_compatibility":false,"encoders":null,"extensions":null,"fmode":"","follow_redirects":false,"headers":{"Content-Type":"application/json","baz":"wibble","foo":"bar"},"ignorebody":false,"ignore_wordlist_comments":false,"inputmode":"","cmd_inputnum":0,"inputproviders":null,"inputshell":"","json":false,"matchers":null,"mmode":"","maxtime":0,"maxtime_job":0,"method":"POST","noninteractive":false,"outputdirectory":"","outputfile":"","outputformat":"","OutputSkipEmptyFile":false,"proxyurl":"","quiet":false,"rate":0,"raw":false,"recursion":false,"recursion_depth":0,"recursion_strategy":"","replayproxyurl":"","requestfile":"","requestproto":"","scraperfile":"","scrapers":"","sni":"","stop_403":false,"stop_all":false,"stop_errors":false,"threads":0,"timeout":0,"url":"http://example.com/aaaa","verbose":false,"wordlists":null,"http2":false,"client-cert":"","client-key":""}}
{"Name":"ffuf.Request","Data":{"Method":"POST","Host":"","Url":"http://example.com/aaaa","Headers":{"Content-Type":"application/json","baz":"wibble","foo":"bar"},"Data":"eyJxdW90ZSI6IkknbGwgc3RpbGwgYmUgaGVyZSB0b21vcnJvdyB0byBoaWdoIGZpdmUgeW91IHllc3RlcmRheSwgbXkgZnJpZW5kLiBQZWFjZS4ifQ==","Input":null,"Position":0,"Raw":"","Timestamp":"0001-01-01T00:00:00Z"}}
`

	headers := make(map[string]string)
	headers["foo"] = "bar"
	headers["baz"] = "wibble"
	headers["Content-Type"] = "application/json"

	data := "{\"quote\":\"I'll still be here tomorrow to high five you yesterday, my friend. Peace.\"}"

	req := ffuf.Request{Method: "POST", Url: "http://example.com/aaaa", Headers: headers, Data: []byte(data)}
	config := ffuf.Config{Method: "POST", Url: "http://example.com/aaaa", Headers: headers, Data: data}

	file, _ := os.CreateTemp("", "prefix")
	filename := file.Name()
	file.Close()
	audit, err := NewAuditLogger(filename)

	if err != nil {
		t.Errorf("Error creating audit logger: %s", err)
	}

	if audit == nil {
		t.Errorf("Audit was nil, expected non-nil")
	}

	// test attempting to log a bung object and erroring
	type A struct{ A *A } // cyclic struct to break JSON Marshalling
	a := A{}
	a.A = &a

	err = audit.Write(a)
	if err == nil {
		t.Errorf("Error was nil, expected non-nil")
	}

	// test writing the config and request objects
	audit.Write(config)
	audit.Write(req)
	audit.Close()

	// re-read and check the audit log
	f, err := os.ReadFile(filename)
	if err != nil {
		t.Errorf("Error reading audit log: %s", err)
	}

	if string(f) != expected {
		t.Errorf("Audit write did not produce the expected logfile.")
	}

	// writing to a closed audit log should fail
	err = audit.Write(config)
	if err == nil {
		t.Errorf("Error was nil, expected non-nil")
	}
}
