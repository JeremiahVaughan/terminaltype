package main

import "time"

func startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 mins
	go func() {
		for {
			<-ticker.C
            // todo implement cleanup
		}
	}()
}
