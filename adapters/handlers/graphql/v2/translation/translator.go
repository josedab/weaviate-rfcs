//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package translation

import (
	"fmt"
	"regexp"
	"strings"
)

// Translator translates v1 GraphQL queries to v2 format
type Translator struct {
	classNames []string
}

// NewTranslator creates a new query translator
func NewTranslator(classNames []string) *Translator {
	return &Translator{
		classNames: classNames,
	}
}

// TranslateV1ToV2 translates a v1 query to v2 format
func (t *Translator) TranslateV1ToV2(v1Query string) (string, error) {
	// Remove Get wrapper
	v2Query := t.removeGetWrapper(v1Query)

	// Convert class names to lowercase and pluralize
	v2Query = t.convertClassNames(v2Query)

	// Convert _additional to _metadata
	v2Query = strings.ReplaceAll(v2Query, "_additional", "_metadata")

	// Wrap results in connection structure
	v2Query = t.wrapInConnection(v2Query)

	// Convert nearVector to near
	v2Query = t.convertVectorSearch(v2Query)

	return v2Query, nil
}

// removeGetWrapper removes the Get { } wrapper from v1 queries
func (t *Translator) removeGetWrapper(query string) string {
	// Remove "Get {" and its closing brace
	re := regexp.MustCompile(`(?s)Get\s*\{(.*)\}`)
	matches := re.FindStringSubmatch(query)
	if len(matches) > 1 {
		return matches[1]
	}
	return query
}

// convertClassNames converts class names to v2 format (lowercase + plural)
func (t *Translator) convertClassNames(query string) string {
	for _, className := range t.classNames {
		// Convert to lowercase first letter
		lowerClassName := t.toLowerFirst(className)
		pluralName := t.pluralize(lowerClassName)

		// Replace class name with plural lowercase version
		// Look for pattern like "Article(" or "Article {"
		re := regexp.MustCompile(className + `\s*(\(|\{)`)
		query = re.ReplaceAllString(query, pluralName+"$1")
	}
	return query
}

// wrapInConnection wraps results in edges { node { } } structure
func (t *Translator) wrapInConnection(query string) string {
	// Find field selections and wrap them in edges { node { } }
	// This is a simplified implementation - real version would parse AST

	for _, className := range t.classNames {
		pluralName := t.pluralize(t.toLowerFirst(className))

		// Look for field selections after class name
		pattern := pluralName + `\s*(\([^)]*\))?\s*\{([^}]+)\}`
		re := regexp.MustCompile(pattern)

		query = re.ReplaceAllStringFunc(query, func(match string) string {
			// Extract arguments and fields
			argMatch := regexp.MustCompile(`\(([^)]*)\)`).FindStringSubmatch(match)
			args := ""
			if len(argMatch) > 1 {
				args = "(" + argMatch[1] + ")"
			}

			// Extract field selections
			fieldMatch := regexp.MustCompile(`\{([^}]+)\}`).FindStringSubmatch(match)
			fields := ""
			if len(fieldMatch) > 1 {
				fields = strings.TrimSpace(fieldMatch[1])
			}

			// Wrap in connection structure
			return fmt.Sprintf("%s%s {\n  edges {\n    node {\n      %s\n    }\n  }\n}",
				pluralName, args, fields)
		})
	}

	return query
}

// convertVectorSearch converts v1 vector search to v2 format
func (t *Translator) convertVectorSearch(query string) string {
	// Convert nearVector to near
	query = strings.ReplaceAll(query, "nearVector:", "near:")

	// Convert nearText to near with text field
	query = strings.ReplaceAll(query, "nearText:", "near:")

	// Convert certainty/distance from _additional to edge level
	// This would require more complex AST manipulation in real implementation

	return query
}

// Helper functions

func (t *Translator) toLowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]|32) + s[1:]
}

func (t *Translator) pluralize(s string) string {
	// Simple pluralization
	if len(s) == 0 {
		return s
	}

	switch s {
	case "person":
		return "people"
	case "category":
		return "categories"
	}

	if s[len(s)-1] == 'y' {
		return s[:len(s)-1] + "ies"
	}
	return s + "s"
}

// TranslateV2ToV1 translates a v2 query back to v1 format (for compatibility)
func (t *Translator) TranslateV2ToV1(v2Query string) (string, error) {
	// Reverse of V1ToV2 translation
	// This would be used for testing and validation

	v1Query := v2Query

	// Convert _metadata back to _additional
	v1Query = strings.ReplaceAll(v1Query, "_metadata", "_additional")

	// Unwrap connection structure
	v1Query = t.unwrapConnection(v1Query)

	// Convert class names back to capitalized singular
	v1Query = t.unconvertClassNames(v1Query)

	// Wrap in Get { }
	v1Query = "{\n  Get {\n" + v1Query + "\n  }\n}"

	return v1Query, nil
}

func (t *Translator) unwrapConnection(query string) string {
	// Remove edges { node { } } wrapper
	re := regexp.MustCompile(`edges\s*\{\s*node\s*\{([^}]+)\}\s*\}`)
	return re.ReplaceAllString(query, "$1")
}

func (t *Translator) unconvertClassNames(query string) string {
	for _, className := range t.classNames {
		pluralName := t.pluralize(t.toLowerFirst(className))

		// Replace plural lowercase with singular capitalized
		re := regexp.MustCompile(pluralName + `\s*(\(|\{)`)
		query = re.ReplaceAllString(query, className+"$1")
	}
	return query
}
