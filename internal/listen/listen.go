// Package listen lets a running handler wait for the next matching message or
// callback query from Telegram. It replaces the dynamic bot.on("message")
// listeners the TypeScript version attached and removed around each wizard
// step: a handler registers a Listener, the controller fans every update out
// to the registered listeners, and a listener that returns true is removed.
package listen

import (
	"context"
	"sync"

	tg "github.com/go-telegram/bot/models"
)

// Listener inspects incoming updates. Each callback returns true when it has
// consumed what it was waiting for and should be unregistered.
type Listener struct {
	OnMessage  func(*tg.Message) bool
	OnCallback func(*tg.CallbackQuery) bool
}

// Registry holds the active listeners.
type Registry struct {
	mu    sync.Mutex
	next  uint64
	items map[uint64]*Listener
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{items: map[uint64]*Listener{}}
}

// Add registers l and returns a function that unregisters it (idempotent).
func (r *Registry) Add(l *Listener) (remove func()) {
	r.mu.Lock()
	id := r.next
	r.next++
	r.items[id] = l
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.items, id)
		r.mu.Unlock()
	}
}

func (r *Registry) snapshot() map[uint64]*Listener {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[uint64]*Listener, len(r.items))
	for id, l := range r.items {
		out[id] = l
	}
	return out
}

// Dispatch offers u to every registered listener. Listeners run on the
// caller's goroutine, so a listener must not block.
func (r *Registry) Dispatch(u *tg.Update) {
	if u == nil {
		return
	}
	for id, l := range r.snapshot() {
		done := false
		switch {
		case u.Message != nil && l.OnMessage != nil:
			done = l.OnMessage(u.Message)
		case u.CallbackQuery != nil && l.OnCallback != nil:
			done = l.OnCallback(u.CallbackQuery)
		}
		if done {
			r.mu.Lock()
			delete(r.items, id)
			r.mu.Unlock()
		}
	}
}

// WaitMessage blocks until accept returns true for an incoming message, or
// ctx ends. accept may have side effects (e.g. nudging the user) and is called
// once per message.
func (r *Registry) WaitMessage(ctx context.Context, accept func(*tg.Message) bool) (*tg.Message, error) {
	ch := make(chan *tg.Message, 1)
	var once sync.Once
	remove := r.Add(&Listener{OnMessage: func(m *tg.Message) bool {
		if !accept(m) {
			return false
		}
		once.Do(func() { ch <- m })
		return true
	}})
	defer remove()
	select {
	case m := <-ch:
		return m, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WaitCallback blocks until accept returns true for a callback query, or ctx
// ends.
func (r *Registry) WaitCallback(ctx context.Context, accept func(*tg.CallbackQuery) bool) (*tg.CallbackQuery, error) {
	ch := make(chan *tg.CallbackQuery, 1)
	var once sync.Once
	remove := r.Add(&Listener{OnCallback: func(q *tg.CallbackQuery) bool {
		if !accept(q) {
			return false
		}
		once.Do(func() { ch <- q })
		return true
	}})
	defer remove()
	select {
	case q := <-ch:
		return q, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
