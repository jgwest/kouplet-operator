package shared

import (
	"fmt"
	"strconv"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_shared")

var logStartTime = time.Now()

// LogDebug ...
func LogDebug(message string) {
	logInnerMsg(nil, message, "debug")
}

// LogInfo ...
func LogInfo(message string) {
	logInnerMsg(nil, message, "info")
}

// LogError ...
func LogError(err error) {
	LogErrorMsg(err, "")
}

// LogErrorMsg ...
func LogErrorMsg(err error, message string) {
	message = "[ERROR] " + message
	logInnerMsg(err, message, "error")
}

// LogSevere ...
func LogSevere(err error) {
	LogSevereMsg(err, "")
}

// LogSevereMsg ...
func LogSevereMsg(err error, message string) {

	message = "[SEVERE] " + message
	logInnerMsg(err, message, "severe")
}

// LogDebugErr ...
func LogDebugErr(err error) {
	LogDebugErrMsg(err, "")
}

// LogDebugErrMsg ...
func LogDebugErrMsg(err error, message string) {
	logInnerMsg(err, message, "debug")
}

func logInnerMsg(err error, message string, typeVal string) {

	message = generateTS() + message

	if err != nil {
		log.Error(err, message, "type", typeVal)
	} else {
		log.Info(message, "type", typeVal)
	}
}

func generateTS() string {

	t := time.Now()

	elapsedTimeInMsecs := (t.UnixNano() / 1000000) - ((logStartTime.UnixNano()) / 1000000)

	elapsedTimeInSeconds := int(elapsedTimeInMsecs / 1000)

	// Convert to 3-place decimal with padding
	elapsedTimeInDecimal := int(elapsedTimeInMsecs%1000) + 1000
	elapsedTimeInDecimalStr := strconv.Itoa(elapsedTimeInDecimal)[1:]

	time := fmt.Sprintf("[%d.%s] ", elapsedTimeInSeconds, elapsedTimeInDecimalStr)

	return time
}
