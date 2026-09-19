package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserID   = "userID"
	ContextUserName = "userName"
)

type Claims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

type Auth struct {
	Secret []byte
}

func NewAuth(secret string) *Auth { return &Auth{Secret: []byte(secret)} }

func (a *Auth) Issue(userID, name string) (string, time.Time, error) {
	exp := time.Now().Add(7 * 24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Name: name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "livepolls",
		},
	})
	signed, err := token.SignedString(a.Secret)
	return signed, exp, err
}

func (a *Auth) parse(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Pinning the algorithm closes the "alg: none" substitution attack.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.Secret, nil
	}, jwt.WithIssuer("livepolls"), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.Subject == "" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// Required rejects the request when the bearer token is missing or bad.
func (a *Auth) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := a.parse(bearer(c))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign in to continue"})
			return
		}
		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextUserName, claims.Name)
		c.Next()
	}
}

// Optional attaches the user when a valid token is present and otherwise lets
// the request through. Voting uses this: anyone can vote, but a signed-in
// voter is identified by account rather than by cookie.
func (a *Auth) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		if claims, err := a.parse(bearer(c)); err == nil {
			c.Set(ContextUserID, claims.Subject)
			c.Set(ContextUserName, claims.Name)
		}
		c.Next()
	}
}

func bearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func UserID(c *gin.Context) string {
	v, _ := c.Get(ContextUserID)
	s, _ := v.(string)
	return s
}

func UserName(c *gin.Context) string {
	v, _ := c.Get(ContextUserName)
	s, _ := v.(string)
	return s
}
