package common

func ToPtr[T any](x T) *T {
	return &x
}

// PtrValueCopy returns a pointer to a copy of the value of the original pointer.
func PtrValueCopy[T any](x *T) *T {
	if x == nil {
		return nil
	}
	xc := *x
	return &xc
}
