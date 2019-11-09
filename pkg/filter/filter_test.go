package filter

import (
	"testing"
)

func TestNewFilterByName(t *testing.T) {
	scf, _ := NewFilterByName("status", "200")
	if _, ok := scf.(*StatusFilter); !ok {
		t.Errorf("Was expecting statusfilter")
	}

	szf, _ := NewFilterByName("size", "200")
	if _, ok := szf.(*SizeFilter); !ok {
		t.Errorf("Was expecting sizefilter")
	}

	wf, _ := NewFilterByName("word", "200")
	if _, ok := wf.(*WordFilter); !ok {
		t.Errorf("Was expecting wordfilter")
	}

	lf, _ := NewFilterByName("line", "200")
	if _, ok := lf.(*LineFilter); !ok {
		t.Errorf("Was expecting linefilter")
	}

	ref, _ := NewFilterByName("regexp", "200")
	if _, ok := ref.(*RegexpFilter); !ok {
		t.Errorf("Was expecting regexpfilter")
	}
}

func TestNewFilterByNameError(t *testing.T) {
	_, err := NewFilterByName("status", "invalid")
	if err == nil {
		t.Errorf("Was expecing an error")
	}
}

func TestNewFilterByNameNotFound(t *testing.T) {
	_, err := NewFilterByName("nonexistent", "invalid")
	if err == nil {
		t.Errorf("Was expecing an error with invalid filter name")
	}
}
