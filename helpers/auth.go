package helpers

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/sliitmozilla/accounts/config"
	"golang.org/x/crypto/bcrypt"
)

//go:embed jwt.secrets.list
var wellKnownSecretsList string

func MustLoadJWTSecret() {
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")

	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET is missing or too short: it must be at least 32 bytes long. Generate one with: openssl rand -base64 32")
	}

	// load well known secrets list
	file, err := os.Open("helpers/jwt.secrets.list")
	if err != nil {
		log.Println(err)
		panic("Error loading well known secrets list. Aborting")
	}
	defer file.Close()

	wellKnownSecrets := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(wellKnownSecretsList))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		wellKnownSecrets[line] = struct{}{}
	}

	if _, isWeak := wellKnownSecrets[jwtSecret]; isWeak {
		log.Fatal("JWT_SECRET found in well known secrets list. Generate one with: openssl rand -base64 32")
	}
}

func HashPassword(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}

func ValidatePassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func GenerateTokens(id, name, email string, roles []string) (accessToken, refreshToken string, err error) {
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	c := config.GetConfig()

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"typ":   "access",
		"id":    id,
		"name":  name,
		"email": email,
		"roles": roles,
		"exp":   time.Now().Add(c.Lifespan.AccessToken * time.Second).Unix(),
		"iat":   time.Now().Unix(),
	})
	accessToken, err = access.SignedString([]byte(jwtSecret))
	if err != nil {
		return
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"typ": "refresh",
		"id":  id,
		"exp": time.Now().Add(c.Lifespan.RefreshToken * time.Second).Unix(),
		"iat": time.Now().Unix(),
	})
	refreshToken, err = refresh.SignedString([]byte(jwtSecret))
	return
}

func GetClaimsFromToken(token string) (jwt.MapClaims, error) {
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	}); err != nil {
		return nil, err
	}
	return claims, nil
}
