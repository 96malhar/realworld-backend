package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var (
	ErrInvalidToken = errors.New("token is invalid")
	ErrExpiredToken = errors.New("token has expired")
)

type JWTMaker struct {
	secretKey     string
	issuer        string
	signingMethod jwt.SigningMethod
}

type Claims struct {
	UserID int64
	jwt.RegisteredClaims
}

// NewJWTMaker creates a new JWTMaker with the given secret key and issuer.
func NewJWTMaker(secretKey string, issuer string) *JWTMaker {
	return &JWTMaker{
		secretKey:     secretKey,
		issuer:        issuer,
		signingMethod: jwt.SigningMethodHS256,
	}
}

// CreateToken generates a new JWT token for the given user ID and duration.
// It signs the token with the secret key and includes the issuer in the claims.
// It uses the HS256 signing method.
func (maker *JWTMaker) CreateToken(userID int64, duration time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    maker.issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(maker.secretKey))
}

// VerifyToken checks the validity of the given token string and returns the claims if valid.
// It returns an error if the token is invalid or expired.
func (maker *JWTMaker) VerifyToken(tokenString string) (*Claims, error) {
	keyFunc := func(token *jwt.Token) (any, error) {
		return []byte(maker.secretKey), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check issuer if expected
	if maker.issuer != "" && claims.Issuer != maker.issuer {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
