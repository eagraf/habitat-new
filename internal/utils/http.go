package utils

import (
	"net/http"

	"github.com/eagraf/habitat-new/internal/node/logging"
)

var (
	log = logging.NewLogger()
)

func LogAndHTTPError(w http.ResponseWriter, error string, code int) {
	log.Error().Msg(error)
	http.Error(w, error, code)
}
