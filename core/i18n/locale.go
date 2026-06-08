// Package i18n picks Russian vs English UI strings from the OS locale.
package i18n

import "strings"

// Russian when the user locale is Russian; English otherwise.
func Locale() string {
	if userLocaleRussian() {
		return "ru"
	}
	return "en"
}

// Pick returns ru when the active locale is Russian, otherwise en.
func Pick(ru, en string) string {
	if Locale() == "ru" {
		return ru
	}
	return en
}

func isRussianTag(tag string) bool {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return false
	}
	base := strings.Split(tag, ".")[0]
	base = strings.Split(base, "_")[0]
	base = strings.Split(base, "-")[0]
	return base == "ru"
}
