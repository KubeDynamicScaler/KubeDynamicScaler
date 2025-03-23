package controller

// ptr returns a pointer to the provided value
func ptr[T any](v T) *T {
	return &v
}
