package errors

import (
	"fmt"
	"strings"
)

type MissingEnvErr struct {
	EnvMap map[string]string
}

func (e MissingEnvErr) Error() string {
	// Get keys of missing environment variables
	missingKeys := make([]string, 0, len(e.EnvMap))
	for key, val := range e.EnvMap {
		if val == "" {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		allKeys := strings.Join(missingKeys, ", ")
		return fmt.Sprintf("insufficient env variables: [%s]", allKeys)
	}
	return "insufficient env variables"
}
