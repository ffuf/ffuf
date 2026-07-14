package filter

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestNewContentTypeFilter(t *testing.T) {
	f, _ := NewContentTypeFilter("application/json,text/html")
	ctf := f.(*ContentTypeFilter)
	if len(ctf.Value) != 2 {
		t.Errorf("ContentTypeFilter should have 2 values, has %d", len(ctf.Value))
	}
}

func TestContentTypeFiltering(t *testing.T) {
	f, _ := NewContentTypeFilter("application/json")
	ctf := f.(*ContentTypeFilter)
	resp := &ffuf.Response{
		ContentType: "application/json; charset=utf-8",
	}
	match, err := ctf.Filter(resp)
	if err != nil {
		t.Errorf("Filter error: %s", err)
	}
	if !match {
		t.Errorf("Should match 'application/json' content type")
	}
}

func TestContentTypeFilteringWildcard(t *testing.T) {
	f, _ := NewContentTypeFilter("application/*")
	ctf := f.(*ContentTypeFilter)
	resp := &ffuf.Response{
		ContentType: "application/xml",
	}
	match, err := ctf.Filter(resp)
	if err != nil {
		t.Errorf("Filter error: %s", err)
	}
	if !match {
		t.Errorf("Should match 'application/*' wildcard")
	}
}

func TestContentTypeFilteringNoMatch(t *testing.T) {
	f, _ := NewContentTypeFilter("application/json")
	ctf := f.(*ContentTypeFilter)
	resp := &ffuf.Response{
		ContentType: "text/html",
	}
	match, err := ctf.Filter(resp)
	if err != nil {
		t.Errorf("Filter error: %s", err)
	}
	if match {
		t.Errorf("Should not match 'text/html' when expecting 'application/json'")
	}
}

func TestContentTypeFilteringSubstring(t *testing.T) {
	f, _ := NewContentTypeFilter("json")
	ctf := f.(*ContentTypeFilter)
	resp := &ffuf.Response{
		ContentType: "application/json",
	}
	match, err := ctf.Filter(resp)
	if err != nil {
		t.Errorf("Filter error: %s", err)
	}
	if !match {
		t.Errorf("Should match 'json' substring in 'application/json'")
	}
}

func TestContentTypeFilteringAll(t *testing.T) {
	f, _ := NewContentTypeFilter("all")
	ctf := f.(*ContentTypeFilter)
	resp := &ffuf.Response{
		ContentType: "text/plain",
	}
	match, err := ctf.Filter(resp)
	if err != nil {
		t.Errorf("Filter error: %s", err)
	}
	if !match {
		t.Errorf("Should match all content types with 'all' keyword")
	}
}
