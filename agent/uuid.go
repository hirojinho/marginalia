package agent

import "github.com/google/uuid"

func newTaskID() string {
	return uuid.NewString()
}
