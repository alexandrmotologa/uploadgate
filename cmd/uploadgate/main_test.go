package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCliVersionFlags(t *testing.T) {
	flags := []string{"--version", "-v", "-V", "version"}
	for _, flag := range flags {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".", flag)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("unexpected error for flag %s: %v, output: %s", flag, err, string(out))
			}
			outStr := string(out)
			if !strings.Contains(outStr, "uploadgate v1.0.0") {
				t.Errorf("expected version output containing 'uploadgate v1.0.0', got: %s", outStr)
			}
		})
	}
}

func TestCliHelpFlags(t *testing.T) {
	flags := []string{"--help", "-h", "help"}
	for _, flag := range flags {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".", flag)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("unexpected error for flag %s: %v, output: %s", flag, err, string(out))
			}
			outStr := string(out)
			if !strings.Contains(outStr, "Usage:") || !strings.Contains(outStr, "uploadgate <command>") {
				t.Errorf("expected usage output, got: %s", outStr)
			}
		})
	}
}
