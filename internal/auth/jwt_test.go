package auth

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestJWTMaker_VerifyToken(t *testing.T) {
	testCases := []struct {
		name        string
		setup       func() (string, *JWTMaker)
		expectedErr error
	}{
		{
			name: "Valid token",
			setup: func() (string, *JWTMaker) {
				tm := NewJWTMaker("some-secret-key", "test-issuer")
				token, _ := tm.CreateToken(1, 5*time.Minute)
				return token, tm
			},
			expectedErr: nil,
		},
		{
			name: "Expired token",
			setup: func() (string, *JWTMaker) {
				tm := NewJWTMaker("some-secret-key", "test-issuer")
				token, _ := tm.CreateToken(1, -5*time.Minute)
				return token, tm
			},
			expectedErr: ErrExpiredToken,
		},
		{
			name: "Invalid secret key",
			setup: func() (string, *JWTMaker) {
				tm := NewJWTMaker("some-secret-key", "test-issuer")
				token, _ := tm.CreateToken(1, 5*time.Minute)
				tm.secretKey = "invalid-secret-key"
				return token, tm
			},
			expectedErr: ErrInvalidToken,
		},
		{
			name: "Invalid issuer",
			setup: func() (string, *JWTMaker) {
				tm := NewJWTMaker("some-secret-key", "test-issuer")
				token, _ := tm.CreateToken(1, 5*time.Minute)
				tm.issuer = "invalid-issuer"
				return token, tm
			},
			expectedErr: ErrInvalidToken,
		},
		{
			name: "Malformed token",
			setup: func() (string, *JWTMaker) {
				tm := NewJWTMaker("some-secret-key", "test-issuer")
				return "malformed.token", tm
			},
			expectedErr: ErrInvalidToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, tm := tc.setup()
			claims, err := tm.VerifyToken(token)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Nil(t, claims)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, int64(1), claims.UserID)
				assert.Equal(t, "test-issuer", claims.Issuer)
			}
		})
	}
}
