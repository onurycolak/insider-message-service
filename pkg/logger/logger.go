package logger

import (
	"log"
)

// Initialize logging flags (called once from main)
func Init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func Infof(format string, v ...any) {
	log.Printf("[INFO] "+format, v...)
}

func Warnf(format string, v ...any) {
	log.Printf("[WARN] "+format, v...)
}

func Errorf(format string, v ...any) {
	log.Printf("[ERROR] "+format, v...)
}

func Debugf(format string, v ...any) {
	log.Printf("[DEBUG] "+format, v...)
}

func Fatalf(format string, v ...any) {
	log.Fatalf("[FATAL] "+format, v...)
}
