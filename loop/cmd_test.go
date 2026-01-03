package loop

import (
	"testing"
)

func TestCmdDoNotPanic(t *testing.T) {
	// This test verifies that calling Cmd.Do doesn't cause infinite loop
	// by checking it completes within reasonable time (test timeout)
	
	done := make(chan bool, 1)
	go func() {
		err := Cmd.Do(Cmd)
		if err != nil {
			t.Errorf("Cmd.Do returned error: %v", err)
		}
		done <- true
	}()
	
	select {
	case <-done:
		// Success - completed without hanging
	}
}

func TestHelpCmdDoNotPanic(t *testing.T) {
	// Test HelpCmd with nil caller doesn't panic or hang
	done := make(chan bool, 1)
	go func() {
		err := HelpCmd.Do(HelpCmd)
		if err != nil {
			t.Errorf("HelpCmd.Do returned error: %v", err)
		}
		done <- true
	}()
	
	select {
	case <-done:
		// Success
	}
}

func TestShowHelp(t *testing.T) {
	// Test showHelp function works
	err := showHelp(Cmd)
	if err != nil {
		t.Errorf("showHelp returned error: %v", err)
	}
}
