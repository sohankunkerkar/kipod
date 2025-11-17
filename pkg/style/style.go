package style

import "fmt"

// Step prints a step with a checkmark
func Step(format string, a ...interface{}) {
	fmt.Printf(" ✓ "+format+"\n", a...)
}

// Info prints an informational message with a bullet point
func Info(format string, a ...interface{}) {
	fmt.Printf(" • "+format+"\n", a...)
}

// Success prints a success message with a bullet point and a heart
func Success(format string, a ...interface{}) {
	fmt.Printf(" • "+format+" 💚\n", a...)
}

// Header prints a header message without a prefix
func Header(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
}
