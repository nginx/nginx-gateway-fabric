package meta

import (
	"time"

	"github.com/google/uuid"
	agentUuid "github.com/nginx/agent/v3/pkg/uuid"
)

const CorrelationIDKey = "correlation_id"

// GenerateMessageID generates a unique message ID, falling back to sha256 and timestamp if UUID generation fails.
func GenerateMessageID() string {
	uuidv7, err := uuid.NewUUID()
	if err != nil {
		return agentUuid.Generate("%s", time.Now().String())
	}

	return uuidv7.String()
}
