package pipe

import (
	"fmt"
	"log"
	"sync"
)

var (
	logChan    = make(chan string, 1024)
	onceLogger sync.Once
)

func initLogger() {
	onceLogger.Do(func() {
		go func() {
			for msg := range logChan {
				log.Println(msg)
			}
		}()
	})
}

func logSizeAsync(label string, payload []byte) {
	initLogger()
	select {
	case logChan <- fmt.Sprintf("%s: %d bytes", label, len(payload)):
	default:
		// Drop log if buffer full to avoid blocking
	}
}

// LogSize is the public helper
func LogSize(label string, payload []byte) {
	if payload != nil {
		logSizeAsync(label, payload)
	}
}
