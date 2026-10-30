package cmd

import "testing"

func TestItemWriter_WriteCommands(t *testing.T) {
	writers := map[string]any{
		"add":      &AddCmd{},
		"start":    &StartCmd{},
		"complete": &CompleteCmd{},
	}

	for name, c := range writers {
		w, ok := c.(ItemWriter)
		if !ok {
			t.Errorf("%s does not implement ItemWriter", name)
			continue
		}

		w.writesItems() // marker method: must not panic
	}
}

func TestItemWriter_ReadCommandsAreNot(t *testing.T) {
	readers := map[string]any{
		"show":  &ShowCmd{},
		"next":  &NextCmd{},
		"ready": &ReadyCmd{},
		"list":  &ListCmd{},
	}

	for name, c := range readers {
		if _, ok := c.(ItemWriter); ok {
			t.Errorf("%s implements ItemWriter but should not", name)
		}
	}
}
