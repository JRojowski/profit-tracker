//go:build !race

package httpapi_test

// raceDetector tells that the test binary has the race detector. See race_on_test.go.
const raceDetector = false
