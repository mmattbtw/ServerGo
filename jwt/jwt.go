package jwt

import (
	"fmt"

	"github.com/SevenTV/ServerGo/configure"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const alg = `{"alg": "HS256","typ": "JWT"}`

func Sign(pl interface{}) (string, error) {
	bytes, err := json.MarshalToString(pl)
	if err != nil {
		return "", err
	}

	algEnc := jwt.EncodeSegment(utils.S2B(alg))
	payload := jwt.EncodeSegment(utils.S2B(bytes))

	first := fmt.Sprintf("%s.%s", algEnc, payload)

	sign, err := jwt.SigningMethodHS256.Sign(first, utils.S2B(configure.Config.GetString("jwt_secret")))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", first, sign), nil
}

func Verify(token []string, out interface{}) error {
	if err := jwt.SigningMethodHS256.Verify(fmt.Sprintf("%s.%s", token[0], token[1]), token[2], utils.S2B(configure.Config.GetString("jwt_secret"))); err != nil {
		return err
	}

	val, err := jwt.DecodeSegment(token[1])
	if err != nil {
		return err
	}

	return json.Unmarshal(val, out)
}
