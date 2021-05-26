package validation

import "regexp"

var (
	emoteNameRegex = regexp.MustCompile(`^[-_A-Za-z():0-9]{2,100}$`)

//	ValidateEmoteTag = regexp.MustCompile(`^[\\w-]{2,100}$`)
)

func ValidateEmoteName(name []byte) bool {
	return emoteNameRegex.Match(name)
}

func ValidateEmoteTag(tag []byte) bool {
	length := len(tag)
	if length < 2 || length > 15 {
		return false
	}
	for _, b := range tag {
		// Ascii characters basically the regex [A-Za-z]
		if (b >= 48 && b <= 57) || (b >= 65 && b <= 90) || (b >= 97 && b <= 122) {
			continue
		}
		return false
	}
	return true
}
