package app

import "strings"

func IsArgumentAccepted(accepted []string, arg string) bool {
	lowerArg := strings.ToLower(arg)
	for _, acceptedArg := range accepted {
		if lowerArg == acceptedArg {
			return true
		}
	}
	return false
}
