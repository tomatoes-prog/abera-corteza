// SPDX-License-Identifier: Apache-2.0

package jwtauth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
)

type JWTAuth struct {
	alg      jwa.SignatureAlgorithm
	verify   interface{}
	verifier jwt.ParseOption
}

type contextKey struct {
	name string
}

var (
	TokenCtxKey = &contextKey{"Token"}
	ErrorCtxKey = &contextKey{"Error"}
)

var (
	ErrUnauthorized = errors.New("token is unauthorized")
	ErrExpired      = errors.New("token is expired")
	ErrNBFInvalid   = errors.New("token nbf validation failed")
	ErrIATInvalid   = errors.New("token iat validation failed")
	ErrNoTokenFound = errors.New("no token found")
)

func New(alg string, signKey interface{}, verifyKey interface{}) *JWTAuth {
	verify := verifyKey
	if verify == nil {
		verify = signKey
	}

	ja := &JWTAuth{
		alg:    jwa.SignatureAlgorithm(alg),
		verify: verify,
	}
	ja.verifier = jwt.WithVerify(ja.alg, ja.verify)

	return ja
}

func VerifyRequest(ja *JWTAuth, r *http.Request, findTokenFns ...func(r *http.Request) string) (jwt.Token, error) {
	for _, fn := range findTokenFns {
		if tokenString := fn(r); tokenString != "" {
			return VerifyToken(ja, tokenString)
		}
	}

	return nil, ErrNoTokenFound
}

func VerifyToken(ja *JWTAuth, tokenString string) (jwt.Token, error) {
	token, err := ja.Decode(tokenString)
	if err != nil {
		return token, errorReason(err)
	}

	if token == nil {
		return nil, ErrUnauthorized
	}

	if err = jwt.Validate(token); err != nil {
		return token, errorReason(err)
	}

	return token, nil
}

func (ja *JWTAuth) Decode(tokenString string) (jwt.Token, error) {
	return jwt.Parse([]byte(tokenString), ja.verifier)
}

func NewContext(ctx context.Context, t jwt.Token, err error) context.Context {
	ctx = context.WithValue(ctx, TokenCtxKey, t)
	ctx = context.WithValue(ctx, ErrorCtxKey, err)
	return ctx
}

func FromContext(ctx context.Context) (jwt.Token, map[string]interface{}, error) {
	token, _ := ctx.Value(TokenCtxKey).(jwt.Token)

	claims := map[string]interface{}{}
	if token != nil {
		var err error
		claims, err = token.AsMap(context.Background())
		if err != nil {
			return token, nil, err
		}
	}

	err, _ := ctx.Value(ErrorCtxKey).(error)
	return token, claims, err
}

func TokenFromHeader(r *http.Request) string {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}

	return header[len(prefix):]
}

func TokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("jwt")
}

func TokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("jwt")
	if err != nil {
		return ""
	}

	return cookie.Value
}

func errorReason(err error) error {
	switch err.Error() {
	case "exp not satisfied", ErrExpired.Error():
		return ErrExpired
	case "iat not satisfied", ErrIATInvalid.Error():
		return ErrIATInvalid
	case "nbf not satisfied", ErrNBFInvalid.Error():
		return ErrNBFInvalid
	default:
		return ErrUnauthorized
	}
}

func (k *contextKey) String() string {
	return "jwtauth context value " + k.name
}
