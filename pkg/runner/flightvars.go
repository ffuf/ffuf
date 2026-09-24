package runner

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// flightResponse is what a by-name variable can be read from: one preflight or
// postflight response. The HTML is parsed at most once, and only when a form or
// meta lookup needs it.
type flightResponse struct {
	body    []byte
	header  http.Header
	doc     *goquery.Document
	docErr  error
	docDone bool
}

func (fr *flightResponse) document() (*goquery.Document, error) {
	if !fr.docDone {
		fr.doc, fr.docErr = goquery.NewDocumentFromReader(bytes.NewReader(fr.body))
		fr.docDone = true
	}
	return fr.doc, fr.docErr
}

// lookup returns the value of key in one source, and whether it was present.
func (fr *flightResponse) lookup(source, key string) (string, bool) {
	switch source {
	case ffuf.VarSourceForm:
		doc, err := fr.document()
		if err != nil {
			return "", false
		}
		return formValue(doc, key)
	case ffuf.VarSourceMeta:
		doc, err := fr.document()
		if err != nil {
			return "", false
		}
		return metaValue(doc, key)
	case ffuf.VarSourceCookie:
		return cookieValue(fr.header, key)
	case ffuf.VarSourceHeader:
		// http.Header.Values canonicalizes the name, so the match is
		// case-insensitive. Only the first value is used.
		if vals := fr.header.Values(key); len(vals) > 0 {
			return vals[0], true
		}
	}
	return "", false
}

// formValue returns the value a browser would submit for the first control named
// key: an input's value attribute (skipping unchecked radios and checkboxes), a
// textarea's text, or a select's selected (else first) option. goquery decodes
// HTML entities, so "a&amp;b" comes back as "a&b", which a regex over the raw
// markup would get wrong.
func formValue(doc *goquery.Document, key string) (string, bool) {
	var val string
	found := false
	doc.Find("input, textarea, select").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if name, _ := s.Attr("name"); name != key {
			return true
		}
		switch goquery.NodeName(s) {
		case "input":
			typ := strings.ToLower(s.AttrOr("type", ""))
			if _, checked := s.Attr("checked"); (typ == "radio" || typ == "checkbox") && !checked {
				return true
			}
			val = s.AttrOr("value", "")
		case "textarea":
			val = s.Text()
		case "select":
			opt := s.Find("option[selected]").First()
			if opt.Length() == 0 {
				opt = s.Find("option").First()
			}
			if v, ok := opt.Attr("value"); ok {
				val = v
			} else {
				val = strings.TrimSpace(opt.Text())
			}
		}
		found = true
		return false
	})
	return val, found
}

// metaValue returns the content of the first <meta name=key>, the way Rails,
// Laravel and Spring publish their CSRF token. Meta names compare
// case-insensitively.
func metaValue(doc *goquery.Document, key string) (string, bool) {
	var val string
	found := false
	doc.Find("meta[name]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if !strings.EqualFold(s.AttrOr("name", ""), key) {
			return true
		}
		val, found = s.AttrOr("content", ""), true
		return false
	})
	return val, found
}

// cookieValue returns the value of the cookie called name out of every
// Set-Cookie header. Header.Get("Set-Cookie") would only see the first one. When
// a response sets the same cookie twice the last one wins, as in a browser, and a
// cookie being deleted (Max-Age<0) counts as absent. The value is returned as
// sent, without percent-decoding.
func cookieValue(header http.Header, name string) (string, bool) {
	var val string
	found := false
	for _, c := range (&http.Response{Header: header}).Cookies() {
		if c.Name != name {
			continue
		}
		val, found = c.Value, c.MaxAge >= 0
	}
	return val, found
}

// extractByName resolves a -preflight-var-auto variable. A pinned source is read
// directly. VarSourceAuto tries every source in ffuf.VarSources order; the first
// response that resolves it pins the source for the rest of the run, so a later
// error page carrying a same-named header can't quietly swap the value. If the
// key is present in two sources with different values, it refuses to guess.
func (r *SimpleRunner) extractByName(ve *ffuf.VarExtract, fr *flightResponse) (string, error) {
	source := ve.Source
	if source == ffuf.VarSourceAuto {
		if pinned, ok := r.autoPins.Load(ve); ok {
			source = pinned.(string)
		}
	}
	if source != ffuf.VarSourceAuto {
		if val, ok := fr.lookup(source, ve.Key); ok {
			return val, nil
		}
		return "", fmt.Errorf("[%s]%s not found", source, ve.Key)
	}

	var picked, val string
	for _, s := range ffuf.VarSources {
		v, ok := fr.lookup(s, ve.Key)
		if !ok {
			continue
		}
		if picked == "" {
			picked, val = s, v
			continue
		}
		if v != val {
			return "", fmt.Errorf("%q is in both [%s] and [%s] with different values; pin one with %s:[%s]%s", ve.Key, picked, s, ve.Name, picked, ve.Key)
		}
	}
	if picked == "" {
		return "", fmt.Errorf("%q not found in any of form, meta, cookie or header", ve.Key)
	}
	pinned, loaded := r.autoPins.LoadOrStore(ve, picked)
	if !loaded {
		log.Printf("preflight: %s resolved to [%s]%s", ve.Name, picked, ve.Key)
	} else if pinned.(string) != picked {
		// Another thread pinned a different source first; follow its pin.
		if v, ok := fr.lookup(pinned.(string), ve.Key); ok {
			return v, nil
		}
		return "", fmt.Errorf("[%s]%s not found", pinned.(string), ve.Key)
	}
	return val, nil
}
