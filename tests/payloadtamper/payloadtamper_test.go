package tests

import (
	"fmt"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/payloadtamper"
)

func Test_payloadTamper(t *testing.T) {
	directory := "./tampers"

	{
		fmt.Println("=== tamper single ===")
		const payload = "' or 1=1 -- "
		pt, err := payloadtamper.New(payloadtamper.Config{
			Directory: directory,
			Tampers:   []string{"space2plus"},
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := pt.LoadTampers(); err != nil {
			t.Fatalf("LoadTampers: %v", err)
		}

		got := pt.Execute(payload)
		expected := "'+or+1=1+--+"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
		fmt.Printf("Success: %s", got)
	}

	{
		fmt.Println("\n=== tampers chained ===")
		const payload = "' or 1=1 -- "
		pt, err := payloadtamper.New(payloadtamper.Config{
			Directory: directory,
			Tampers:   []string{"quote2doublequote", "space2plus", "yeswehack"},
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := pt.LoadTampers(); err != nil {
			t.Fatalf("LoadTampers: %v", err)
		}

		got := pt.Execute(payload)
		expected := "yeswehack_\"+or+1=1+--+"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
		fmt.Printf("Success: %s", got)
	}
	{
		fmt.Println("\n=== tampers info ===")
		infos, err := payloadtamper.GetTampersInfo(directory)
		if err != nil {
			t.Fatalf("TampersList: %v", err)
		}
		if len(infos) == 0 {
			t.Fatal("expected at least one tamper")
		}
		t.Logf("Found %d tampers:", len(infos))
		for _, info := range infos {
			t.Logf("  %s: %s", info.Name, info.Desc)
		}
	}
}
