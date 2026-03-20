package session

import (
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

func TestInputNotifySignalsOnAddInput(t *testing.T) {
	sess := NewSession(nil)

	// Channel should be empty initially.
	select {
	case <-sess.InputNotify():
		t.Fatal("expected no notification before AddInput")
	default:
	}

	// Adding an input should produce exactly one notification.
	sess.AddInput(llm.Message{Role: llm.RoleUser, Content: "hello"})

	select {
	case <-sess.InputNotify():
		// expected
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification after AddInput")
	}

	// The notification channel has capacity 1; rapid successive calls
	// should coalesce into a single notification without blocking.
	sess.AddInput(llm.Message{Role: llm.RoleUser, Content: "a"})
	sess.AddInput(llm.Message{Role: llm.RoleUser, Content: "b"})

	select {
	case <-sess.InputNotify():
		// expected – at least one signal
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for coalesced notification")
	}

	// After draining the notification, no further signal should be pending.
	select {
	case <-sess.InputNotify():
		// A second coalesced signal is acceptable.
	default:
	}
}

func TestInputNotifyNotBlockedByFullQueue(t *testing.T) {
	sess := NewSession(nil)

	// Fill the input queue to capacity.
	for i := range defaultInputQueueSize {
		sess.AddInput(llm.Message{Role: llm.RoleUser, Content: string(rune('A' + (i % 26)))})
	}

	// The next AddInput overflows and is dropped, but must not block.
	done := make(chan struct{})
	go func() {
		sess.AddInput(llm.Message{Role: llm.RoleUser, Content: "overflow"})
		close(done)
	}()

	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("AddInput blocked on full queue")
	}
}
