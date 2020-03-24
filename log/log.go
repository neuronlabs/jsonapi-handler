package log

import (
	"github.com/neuronlabs/neuron-core/log"
)

var logger = log.NewModuleLogger("JSONAPI-HANDLER")

// log levels consts
const (
	LDEBUG3   = log.LDEBUG3
	LDEBUG2   = log.LDEBUG2
	LDEBUG    = log.LDEBUG
	LINFO     = log.LINFO
	LWARNING  = log.LWARNING
	LERROR    = log.LERROR
	LCRITICAL = log.LCRITICAL
)

var (
	// SetLevel is the set level function
	SetLevel = logger.SetLevel
	// Level is the current module log level.
	Level = logger.Level
	// Debug3f writes the formated debug log.
	Debug3f = logger.Debug3f
	// Debug2f writes the formated debug log.
	Debug2f = logger.Debug2f
	// Debugf writes the formated debug log.
	Debugf = logger.Debugf

	// Infof writes the formated info log.
	Infof = logger.Infof

	// Warningf writes the formated warning log.
	Warningf = logger.Warningf

	// Errorf writes the formated error log.
	Errorf = logger.Errorf

	// Fatalf writes the formated fatal log.
	Fatalf = logger.Fatalf

	// Panicf writes the formated panic log.
	Panicf = logger.Panicf

	// Debug3 writes the debug3 level logs.
	Debug3 = logger.Debug3

	// Debug2 writes the debug2 level logs.
	Debug2 = logger.Debug2

	// Debug writes the  debug log.
	Debug = logger.Debug

	// Info writes the info log.
	Info = logger.Info

	// Warning writes the warning log.
	Warning = logger.Warning

	// Error writes the error log.
	Error = logger.Error

	// Fatal writes the fatal log.
	Fatal = logger.Fatal

	// Panic writes the panic log.
	Panic = logger.Panic
)
