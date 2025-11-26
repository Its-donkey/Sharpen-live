package handlers

import (
	"context"
	"fmt"

	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
)

func logWebsub(ctx context.Context, logger logging.Logger, format string, args ...any) {
	if logger == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	logger.Printf("%s", message)
	if id := logging.RequestIDFromContext(ctx); id != "" {
		logging.LogWithID(logger, "websub", id, message)
	} else {
		logging.LogWithID(logger, "websub", "", message)
	}
}
