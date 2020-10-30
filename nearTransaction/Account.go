package nearTransaction

import (
	"encoding/hex"
	"regexp"
)

const pattern = `^(([a-z\d]+[\-_])*[a-z\d]+\.)*([a-z\d]+[\-_])*[a-z\d]+$`

func IsValid(account string) bool {
	if len(account) < 2 || len(account) > 64 {
		return false
	}

	if len(account) == 64 {
		_, err := hex.DecodeString(account)
		if err != nil {
			return false
		}
	}

	matched, err := regexp.MatchString(pattern, account)

	return matched && err == nil
}