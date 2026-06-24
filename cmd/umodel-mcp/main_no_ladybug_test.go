//go:build !ladybug

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDefaultLadybugUnavailableGuidesLocalDevelopment(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run([]string{"--manifest", "--data", t.TempDir()}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("expected default local.ladybug stub to be unavailable in no-ladybug build")
	}
	for _, want := range []string{"local.ladybug", "-tags ladybug", "--graphstore file.memory"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error %q", want, err.Error())
		}
	}
}
