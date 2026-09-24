package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

const flightVarsPage = `<!doctype html><html><head>
<meta name="CSRF-Token" content="meta-tok">
<meta name="shared" content="same">
</head><body>
<form method="POST">
<input type="hidden" name="csrf_token" value="a&amp;b+c/d=">
<input type="hidden" name="data[_Token][key]" value="php-tok">
<input value="attr-first" name="loginForm:token" type="hidden">
<input type="checkbox" name="remember" value="unchecked-box">
<input type="checkbox" name="remember" value="checked-box" checked>
<textarea name="note">text &lt;area&gt;</textarea>
<select name="lang"><option value="en">English</option><option value="fi" selected>Finnish</option></select>
<select name="plain"><option>First</option><option>Second</option></select>
<input type="hidden" name="dup" value="form-dup">
<input type="hidden" name="shared" value="same">
<input type="hidden" name="evil" value="x&#13;&#10;Injected: 1">
</form></body></html>`

// flightVarsServer serves flightVarsPage with a mix of cookies and headers.
// "/noform" serves the same headers without the HTML.
func flightVarsServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Set-Cookie", "sessionid=sess-1; Path=/; HttpOnly")
		h.Add("Set-Cookie", "XSRF-TOKEN=old; Path=/")
		h.Add("Set-Cookie", "XSRF-TOKEN=tok%3D%3D; Path=/")
		h.Add("Set-Cookie", "gone=bye; Max-Age=0")
		h.Add("Set-Cookie", "dup=cookie-dup; Path=/")
		h.Set("X-Csrf-Token", "hdr-tok")
		if r.URL.Path == "/noform" {
			// Only here, so it never collides with the form field of the same name.
			h.Set("Csrf_Token", "hdr-for-pin-test")
			return
		}
		fmt.Fprint(w, flightVarsPage)
	}))
}

// runVar runs a one-var preflight chain against srv and returns the value.
func runVar(t *testing.T, r *SimpleRunner, srv *httptest.Server, path string, ve ffuf.VarExtract) (string, error) {
	t.Helper()
	reqFile := writeTempRequest(t, fmt.Sprintf("GET %s HTTP/1.1\nHost: %s\n\n", path, srv.Listener.Addr().String()))
	chain := []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: []ffuf.VarExtract{ve}}}
	vars, err := r.runPreflightChain(chain, nil)
	return vars[ve.Name], err
}

func TestFlightVarsPinnedSources(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()

	cases := []struct {
		source, key, want string
	}{
		// Entities decoded, base64 characters left alone.
		{ffuf.VarSourceForm, "csrf_token", "a&b+c/d="},
		{ffuf.VarSourceForm, "data[_Token][key]", "php-tok"},
		{ffuf.VarSourceForm, "loginForm:token", "attr-first"},
		{ffuf.VarSourceForm, "remember", "checked-box"},
		{ffuf.VarSourceForm, "note", "text <area>"},
		{ffuf.VarSourceForm, "lang", "fi"},
		{ffuf.VarSourceForm, "plain", "First"},
		{ffuf.VarSourceMeta, "csrf-token", "meta-tok"},
		{ffuf.VarSourceCookie, "sessionid", "sess-1"},
		// Last Set-Cookie for a name wins; the value is not percent-decoded.
		{ffuf.VarSourceCookie, "XSRF-TOKEN", "tok%3D%3D"},
		{ffuf.VarSourceHeader, "x-csrf-token", "hdr-tok"},
	}
	for _, c := range cases {
		t.Run(c.source+"/"+c.key, func(t *testing.T) {
			r := newTestRunner(newTestConfig(srv.URL))
			got, err := runVar(t, r, srv, "/", ffuf.VarExtract{Name: "V", Source: c.source, Key: c.key})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestFlightVarsNotFound(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()
	for _, ve := range []ffuf.VarExtract{
		{Name: "V", Source: ffuf.VarSourceForm, Key: "nope"},
		{Name: "V", Source: ffuf.VarSourceMeta, Key: "csrf_token"},
		// A cookie being deleted is not a value.
		{Name: "V", Source: ffuf.VarSourceCookie, Key: "gone"},
		{Name: "V", Source: ffuf.VarSourceHeader, Key: "X-Missing"},
		{Name: "V", Source: ffuf.VarSourceAuto, Key: "nope"},
	} {
		r := newTestRunner(newTestConfig(srv.URL))
		if _, err := runVar(t, r, srv, "/", ve); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("%+v: error = %v, want not found", ve, err)
		}
	}
}

// The resolution order is form, meta, cookie, header.
func TestFlightVarsAutoOrder(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()
	cases := map[string]string{
		"csrf_token":   "a&b+c/d=", // form beats the Csrf_Token header
		"csrf-token":   "meta-tok",
		"sessionid":    "sess-1",
		"X-Csrf-Token": "hdr-tok",
		"shared":       "same", // in form and meta with equal values: form, no error
	}
	for key, want := range cases {
		r := newTestRunner(newTestConfig(srv.URL))
		got, err := runVar(t, r, srv, "/", ffuf.VarExtract{Name: "V", Source: ffuf.VarSourceAuto, Key: key})
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", key, got, err, want)
		}
	}
}

// Same key, different values in two sources: refuse and say how to pin.
func TestFlightVarsAutoAmbiguous(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()
	r := newTestRunner(newTestConfig(srv.URL))
	_, err := runVar(t, r, srv, "/", ffuf.VarExtract{Name: "DUP", Source: ffuf.VarSourceAuto, Key: "dup"})
	if err == nil || !strings.Contains(err.Error(), "pin one with DUP:[form]dup") {
		t.Fatalf("error = %v, want an ambiguity error suggesting DUP:[form]dup", err)
	}
}

// Once a bare key resolves, the source is pinned: a later response without the
// form must not quietly fall back to a same-named header.
func TestFlightVarsAutoPinHolds(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()
	r := newTestRunner(newTestConfig(srv.URL))
	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	noForm := writeTempRequest(t, fmt.Sprintf("GET /noform HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	chain := []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: []ffuf.VarExtract{{Name: "V", Source: ffuf.VarSourceAuto, Key: "csrf_token"}}}}

	if vars, err := r.runPreflightChain(chain, nil); err != nil || vars["V"] != "a&b+c/d=" {
		t.Fatalf("first run: %v, %v", vars, err)
	}
	chain[0].RequestFile = noForm // same VarExtract, response now lacks the form
	_, err := r.runPreflightChain(chain, nil)
	if err == nil || !strings.Contains(err.Error(), "[form]csrf_token not found") {
		t.Fatalf("error = %v, want the pinned [form] source to be enforced", err)
	}
	// A fresh runner has no pin and resolves the header.
	fresh := newTestRunner(newTestConfig(srv.URL))
	if vars, err := fresh.runPreflightChain(chain, nil); err != nil || vars["V"] != "hdr-for-pin-test" {
		t.Errorf("fresh runner: %v, %v; want the header value", vars, err)
	}
}

// A value with CR/LF is refused whatever source it came from; an entity-encoded
// newline in a form field decodes to a real one.
func TestFlightVarsControlCharsRefused(t *testing.T) {
	srv := flightVarsServer()
	defer srv.Close()
	r := newTestRunner(newTestConfig(srv.URL))
	_, err := runVar(t, r, srv, "/", ffuf.VarExtract{Name: "V", Source: ffuf.VarSourceForm, Key: "evil"})
	if err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("error = %v, want a control character refusal", err)
	}
	r.config.PreflightError = "ignore"
	vars, err := r.runPreflightChain([]ffuf.PreflightConfig{{
		RequestFile: writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String())),
		Vars:        []ffuf.VarExtract{{Name: "V", Source: ffuf.VarSourceForm, Key: "evil"}},
	}}, nil)
	if err != nil {
		t.Fatalf("ignore mode: %s", err)
	}
	if _, ok := vars["V"]; ok {
		t.Error("ignore mode: control-character value was still set")
	}
}

// A by-name value is still a captured value: it can't move a later flight.
func TestFlightVarsCannotChangeDestination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next", "http://attacker.example.invalid/steal")
	}))
	defer srv.Close()
	r := destRunner(t, srv.URL+"/FUZZ", false)
	chain := []ffuf.PreflightConfig{
		{
			RequestFile: writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String())),
			Vars:        []ffuf.VarExtract{{Name: "NEXT", Source: ffuf.VarSourceHeader, Key: "X-Next"}},
		},
		{RequestFile: writeTempRequest(t, "GET NEXT HTTP/1.1\n\n")},
	}
	_, err := r.runPreflightChain(chain, nil)
	if err == nil || !strings.Contains(err.Error(), "changed the request destination") {
		t.Fatalf("error = %v, want the destination guard to refuse", err)
	}
}

// Concurrent first resolutions of one auto var agree on a single pin.
func TestFlightVarsAutoPinConcurrent(t *testing.T) {
	r := newTestRunner(newTestConfig("http://unused"))
	ve := &ffuf.VarExtract{Name: "V", Source: ffuf.VarSourceAuto, Key: "tok"}
	withForm := &flightResponse{body: []byte(`<input name="tok" value="same">`), header: http.Header{}}
	withHeader := &flightResponse{header: http.Header{"Tok": {"same"}}}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		fr := withForm
		if i%2 == 1 {
			fr = withHeader
		}
		wg.Add(1)
		go func(fr *flightResponse) {
			defer wg.Done()
			_, _ = r.extractByName(ve, &flightResponse{body: fr.body, header: fr.header})
		}(fr)
	}
	wg.Wait()
	pinned, ok := r.autoPins.Load(ve)
	if !ok || (pinned != ffuf.VarSourceForm && pinned != ffuf.VarSourceHeader) {
		t.Fatalf("pin = %v, %v", pinned, ok)
	}
}

// After the pin, a response that also carries a different same-named value in a
// lower-priority source reads the pinned source instead of failing as ambiguous.
func TestFlightVarsAutoPinSkipsAmbiguity(t *testing.T) {
	r := newTestRunner(newTestConfig("http://unused"))
	ve := &ffuf.VarExtract{Name: "V", Source: ffuf.VarSourceAuto, Key: "tok"}
	first := &flightResponse{body: []byte(`<input name="tok" value="f1">`), header: http.Header{}}
	if v, err := r.extractByName(ve, first); err != nil || v != "f1" {
		t.Fatalf("first: %q, %v", v, err)
	}
	both := &flightResponse{body: []byte(`<input name="tok" value="f2">`), header: http.Header{"Tok": {"h2"}}}
	if v, err := r.extractByName(ve, both); err != nil || v != "f2" {
		t.Errorf("pinned: got %q, %v; want f2 from [form]", v, err)
	}
}
