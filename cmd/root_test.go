package cmd

import (
	"context"
	"strings"
	"testing"
)

// TestExecute_Version verifies --version prints a version string.
func TestExecute_Version(t *testing.T) {
	// Execute with --version should not return an error (it prints and exits).
	err := Execute(context.Background(), []string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExecute_Help verifies --help (and no args) returns without error.
func TestExecute_Help(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {}} {
		name := "no-args"
		if len(args) > 0 {
			name = args[0]
		}
		t.Run(name, func(t *testing.T) {
			err := Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestExecute_DryRun verifies --dry-run validates and exits cleanly.
func TestExecute_DryRun(t *testing.T) {
	err := Execute(context.Background(), []string{
		"-l", "-p", "8080", "--dry-run",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExecute_DryRunInvalid verifies --dry-run still catches bad configs.
func TestExecute_DryRunInvalid(t *testing.T) {
	err := Execute(context.Background(), []string{
		"-l", "--dry-run", // listen without -p
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// TestExecute_InvalidFlags verifies unknown flags produce an error.
func TestExecute_InvalidFlags(t *testing.T) {
	err := Execute(context.Background(), []string{"--nonexistent-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// TestExecute_ConflictingFlags verifies -e and -c conflict is caught.
func TestExecute_ConflictingFlags(t *testing.T) {
	err := Execute(context.Background(), []string{
		"-e", "cat", "-c", "ls", "localhost", "80", "--dry-run",
	})
	if err == nil {
		t.Fatal("expected error for -e and -c conflict")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutually exclusive: %v", err)
	}
}
