// Package validate is a tiny library exercised in test mode. In test mode
// archbench records each test's pass/fail/skip status instead of benchmark
// metrics, and `archbench compare` reports where that status diverges across
// targets.
package validate

import "errors"

// NonEmpty returns an error when name is empty.
func NonEmpty(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	return nil
}
