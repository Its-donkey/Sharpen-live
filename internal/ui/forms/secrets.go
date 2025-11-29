package forms

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func GenerateHubSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("hub-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
