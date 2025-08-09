package logger

import "context"

var contextKeyRegistry = make(map[interface{}]string)

func RegisterContextKey(ctxKey interface{}, logField string) {
	contextKeyRegistry[ctxKey] = logField
}

func UnregisterContextKey(ctxKey interface{}) {
	delete(contextKeyRegistry, ctxKey)
}

func listContextKeys() map[interface{}]string {
	// Return a copy to prevent external modification
	copy := make(map[interface{}]string, len(contextKeyRegistry))
	for k, v := range contextKeyRegistry {
		copy[k] = v
	}
	return copy
}

func withContext(ctx context.Context) []any {
	fields := make([]any, 0, len(contextKeyRegistry)*2)
	for key, fieldName := range contextKeyRegistry {
		if val := ctx.Value(key); val != nil {
			fields = append(fields, fieldName, val)
		}
	}
	return fields
}
