package httpx

import (
	"errors"
	"fmt"
)

var ErrBodyClose = fmt.Errorf("body could not be closed")

type errBodyCloser struct {
	next error
}

func (e errBodyCloser) Is(target error) bool {
	if errors.Is(target, ErrBodyClose) {
		return true
	}
	return errors.Is(e.next, target)
}

func (e errBodyCloser) Error() string {
	return fmt.Sprintf("%s: %s", ErrBodyClose, e.next.Error())
}
