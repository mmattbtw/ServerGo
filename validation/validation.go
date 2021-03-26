package validation

// var (
// 	emoteNameRegex = regexp.MustCompile(`^[\\w-]{2,100}$`)
// 	ValidateEmoteTag = regexp.MustCompile(`^[\\w-]{2,100}$`)
// )

// TODO: Check hate speech.

func ValidateEmoteName(name []byte) bool {
	length := len(name)
	if length < 2 || length > 100 {
		return false
	}
	for _, b := range name {
		// Ascii characters basically the regex [\w-]
		if b == 45 || b == 95 || (b >= 48 && b <= 57) || (b >= 65 && b <= 90) || (b >= 97 && b <= 122) {
			continue
		}
		return false
	}
	return true
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

func ValidateEmoteVisibility(visibility int32) bool {
	return visibility >= 0 && visibility <= 2
}
