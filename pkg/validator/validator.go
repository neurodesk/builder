package validator

import (
	"fmt"
	"slices"
	"strings"
)

func All(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

type Validatable interface {
	Validate() error
}

func Each[T Validatable](items []T) error {
	for i, item := range items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
	}
	return nil
}

func Map[T any](items []T, f func(T, string) error, description string) error {
	for i, item := range items {
		if err := f(item, fmt.Sprintf("%s[%d]", description, i)); err != nil {
			return err
		}
	}
	return nil
}

func MapDict[T any](items map[string]T, f func(string, T) error, description string) error {
	for key, item := range items {
		if err := f(key, item); err != nil {
			return err
		}
	}
	return nil
}

func NotEmpty(field, description string) error {
	if field == "" {
		return fmt.Errorf("%s must not be empty", description)
	}
	return nil
}

func NoDuplicates[T comparable](slice []T, description string) error {
	seen := make(map[T]struct{})
	for _, v := range slice {
		if _, ok := seen[v]; ok {
			return fmt.Errorf("%s contains duplicate value: %v", description, v)
		}
		seen[v] = struct{}{}
	}
	return nil
}

func SliceHasElements[T comparable](slice []T, allowed []T, description string) error {
	for _, v := range slice {
		if err := MatchesAllowed(v, allowed, description); err != nil {
			return err
		}
	}
	return nil
}

func MatchesAllowed[T comparable](field T, allowed []T, description string) error {
	if !slices.Contains(allowed, field) {
		return fmt.Errorf("%s must be one of %v, got %v", description, allowed, field)
	}
	return nil
}

func HasNoJinja(field string, description string) error {
	if field != "" && (strings.Contains(field, "{{") || strings.Contains(field, "{%")) {
		return fmt.Errorf("%s must not contain jinja templating", description)
	}
	return nil
}
