package common

import (
	"fmt"
	"regexp"
)

const (
	usernamePattern  = `^[A-Za-z0-9_.][A-Za-z0-9_.-]{0,31}$`
	groupnamePattern = `^[A-Za-z0-9_][A-Za-z0-9_-]{0,31}$`
)

// Check that the given name is a valid unix login name
func ValidateUsername(name string) error {
	usernameRegex := regexp.MustCompile(usernamePattern)
	if !usernameRegex.MatchString(name) {
		return fmt.Errorf("user name %q doesn't conform to schema (%s)", name, usernameRegex.String())
	}
	return nil
}

// Check that the given name is a valid unix group name
func ValidateGroupname(name string) error {
	groupnameRegex := regexp.MustCompile(groupnamePattern)
	if !groupnameRegex.MatchString(name) {
		return fmt.Errorf("group name %q doesn't conform to schema (%s)", name, groupnameRegex.String())
	}
	return nil
}
