package fetch

import "fmt"

func WrapError(context string, err error) error {
	return fmt.Errorf("%s: %w", context, err)
}
