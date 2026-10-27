//go:build race

package httpapi_test

// raceDetector tells that the test binary has the race detector. The race detector makes the code
// several times slower, so the tests do not check a time limit with it.
const raceDetector = true
