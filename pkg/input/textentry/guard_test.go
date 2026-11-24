package textentry

import (
	"context"
	"testing"
)

func TestGuardRunsAtQueuedExecutionAndRejectsPassword(t *testing.T) {
	for _, password := range []bool{false, true} {
		c := &Controller{}
		queued := make(chan func(), 1)
		edits := 0
		allowed := true
		c.Attach(func(f func()) { queued <- f }, func(r Request) (View, error) {
			if r.Action != "" {
				edits++
			}
			return View{Password: password}, nil
		})
		done := make(chan error, 1)
		go func() {
			_, err := c.CallGuarded(context.Background(), Request{Action: "type", Text: "x"}, func(v View) bool { return allowed && !v.Password })
			done <- err
		}()
		f := <-queued
		if !password {
			allowed = false
		}
		f()
		if <-done == nil || edits != 0 {
			t.Fatal("queued policy bypass")
		}
	}
}
func TestGuardedEditUsesExistingInputHandler(t *testing.T) {
	c := &Controller{}
	edits := 0
	c.Attach(func(f func()) { f() }, func(r Request) (View, error) {
		if r.Action == "type" {
			edits++
		}
		return View{}, nil
	})
	if _, err := c.CallGuarded(context.Background(), Request{Action: "type", Text: "x"}, func(View) bool { return true }); err != nil || edits != 1 {
		t.Fatal("edit failed")
	}
}
