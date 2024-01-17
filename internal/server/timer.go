package server

import "time"

func stopTimer(t *time.Timer) {
	if !t.Stop() {
		// Collect possibly added time from the channel
		// if timer has been stopped and nobody collected its value.
		select {
		case <-t.C:
		default:
		}
	}
}
