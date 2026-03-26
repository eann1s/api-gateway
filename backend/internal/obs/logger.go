package obs

import (
	"os"

	"github.com/rs/zerolog"
)


func NewLogger(levelStr string, versionStr string) (zerolog.Logger, error) {
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		return zerolog.Logger{}, err
	}
	log := zerolog.
		New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Str("service", "gateway").
		Str("version", versionStr).
		Logger()
	return log, nil
}