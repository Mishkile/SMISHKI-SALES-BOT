package listen

import (
	"context"
	"testing"
	"time"

	tg "github.com/go-telegram/bot/models"
)

func TestWaitMessageFiltersAndConsumes(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := make(chan *tg.Message, 1)
	go func() {
		m, err := r.WaitMessage(ctx, func(m *tg.Message) bool { return m.Text == "yes" })
		if err != nil {
			t.Errorf("wait: %v", err)
		}
		got <- m
	}()

	// Give the waiter time to register.
	deadline := time.Now().Add(time.Second)
	for {
		if len(r.snapshot()) == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	r.Dispatch(&tg.Update{Message: &tg.Message{Text: "no"}})
	r.Dispatch(&tg.Update{Message: &tg.Message{Text: "yes", ID: 7}})
	m := <-got
	if m == nil || m.ID != 7 {
		t.Fatalf("got %+v", m)
	}
	if n := len(r.snapshot()); n != 0 {
		t.Fatalf("listener not removed: %d left", n)
	}
}

func TestWaitCancelled(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.WaitCallback(ctx, func(*tg.CallbackQuery) bool { return true }); err == nil {
		t.Fatal("expected context error")
	}
	if n := len(r.snapshot()); n != 0 {
		t.Fatalf("listener leaked: %d", n)
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	r := NewRegistry()
	remove := r.Add(&Listener{})
	remove()
	remove()
	if n := len(r.snapshot()); n != 0 {
		t.Fatalf("expected empty registry, got %d", n)
	}
}
