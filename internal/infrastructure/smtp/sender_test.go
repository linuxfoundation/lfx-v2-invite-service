// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package smtp

import (
	"strings"
	"testing"
)

func TestSanitizeHeader_RemovesCarriageReturn(t *testing.T) {
	got := sanitizeHeader("user@example.com\rX-Injected: evil")
	if strings.Contains(got, "\r") {
		t.Errorf("sanitizeHeader did not strip \\r: %q", got)
	}
}

func TestSanitizeHeader_RemovesNewline(t *testing.T) {
	got := sanitizeHeader("user@example.com\nX-Injected: evil")
	if strings.Contains(got, "\n") {
		t.Errorf("sanitizeHeader did not strip \\n: %q", got)
	}
}

func TestSanitizeHeader_RemovesCRLF(t *testing.T) {
	got := sanitizeHeader("user@example.com\r\nX-Injected: evil")
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("sanitizeHeader did not strip CRLF: %q", got)
	}
}

func TestSanitizeHeader_PreservesNormalInput(t *testing.T) {
	input := "alice@example.com"
	got := sanitizeHeader(input)
	if got != input {
		t.Errorf("sanitizeHeader modified clean input: got %q, want %q", got, input)
	}
}

func TestSanitizeHeader_EmptyString(t *testing.T) {
	got := sanitizeHeader("")
	if got != "" {
		t.Errorf("sanitizeHeader returned non-empty for empty input: %q", got)
	}
}

func TestSanitizeHeader_PreservesSpecialCharsOtherThanCRLF(t *testing.T) {
	input := "display name <user+tag@example.com>"
	got := sanitizeHeader(input)
	if got != input {
		t.Errorf("sanitizeHeader modified valid header chars: got %q, want %q", got, input)
	}
}
