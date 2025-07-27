package auth

import (
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

func generateNonce() string {
	return base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
}

func doesHandleBelongToDomain(handle string, domain string) bool {
	return strings.HasSuffix(handle, "."+domain) || handle == domain
}
