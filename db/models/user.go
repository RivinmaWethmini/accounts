package models

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sliitmozilla/accounts/db"
	apiErrors "github.com/sliitmozilla/accounts/errors"
	helpers "github.com/sliitmozilla/accounts/helpers"
)

type UserModel struct {
	ID                         uuid.UUID         `json:"id" example:"00000000-0000-0000-0000-000000000000"`
	Name                       string            `json:"name" example:"sliitmozillian"`
	Email                      string            `json:"email" example:"infosliitmcc@gmail.com"`
	Password                   string            `json:"password,omitempty" db:"-"`
	Private                    bool              `json:"private"`
	IsVerified                 bool              `json:"isVerified" db:"is_verified"`
	VerificationToken          string            `json:"verificationToken,omitempty" db:"verification_token"`
	VerificationTokenExpiresAt *time.Time        `json:"verificationTokenExpiresAt,omitempty" db:"verification_token_expires_at"`
	CreatedAt                  *time.Time        `json:"createdAt" example:"2025-12-31T00:00:00Z"`
	UpdatedAt                  *time.Time        `json:"updatedAt" example:"2025-12-31T00:00:00Z"`
	Roles                      []string          `json:"roles" example:"admin"`
	Connections                []ConnectionModel `json:"connections" db:"-"`
}

func (UserModel) Login(email string, password string) (accessToken, refreshToken string, err error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return "", "", err
	}
	defer conn.Close(context.Background())

	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	rows, err := conn.Query(context.Background(),
		`SELECT u.id, u.name, u.password, u.is_verified, array_agg(ur.rolename) AS roles
		FROM users u
		LEFT JOIN userroles ur ON u.id = ur.userid
		WHERE u.email = $1
		GROUP BY u.id`,
		email,
	)
	if err != nil {
		return "", "", err
	}
	if !rows.Next() {
		return "", "", apiErrors.NotFoundError{Msg: "User not found"}
	}

	u := UserModel{Email: email}
	var roles pgtype.Array[pgtype.Text]
	if err := rows.Scan(&u.ID, &u.Name, &u.Password, &u.IsVerified, &roles); err != nil {
		return "", "", err
	}
	// assign roles
	u.Roles = []string{}
	for _, role := range roles.Elements {
		if role.String != "" {
			u.Roles = append(u.Roles, role.String)
		}
	}

	if helpers.ValidatePassword(u.Password, password) {
		if !u.IsVerified {
			return "", "", apiErrors.UnverifiedEmailError{Msg: "Email not verified. Please verify your email before logging in."}
		}
		accessToken, refreshToken, err = helpers.GenerateTokens(u.ID.String(), u.Name, u.Email, u.Roles)
		return
	}
	return "", "", err
}

func (UserModel) SelectAll() ([]UserModel, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(),
		`SELECT u.id, u.name, u.email, u.createdAt, u.updatedAt, u.private, u.is_verified AS "isVerified", array_remove(array_agg(ur.rolename), NULL) AS roles
		FROM users u
		LEFT JOIN userroles ur ON u.id = ur.userid
		GROUP BY u.id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[UserModel])
}

func (UserModel) GetUserByID(id uuid.UUID) (u UserModel, err error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(),
		`SELECT
    	u.id,
    	u.name,
    	u.email,
    	u.createdat,
    	u.updatedat,
    	u.private,
    	u.is_verified,
    	array_agg(DISTINCT ur.rolename) AS roles,
    	(SELECT json_agg(json_build_object(
            'provider', uc.provider,
            'providerUserId', uc.provideruserid,
            'providerUserName', uc.providerusername,
            'providerAccountEmail', uc.provideraccountemail,
            'linkedAt', to_char(uc.linkedat, 'YYYY-MM-DD"T"HH24:MI:SSZ')
        ))
		FROM userconnections uc
      	WHERE uc.userid = u.id
    	) AS connections
		FROM users u
		LEFT JOIN userroles ur ON u.id = ur.userid
		WHERE u.id = $1
		GROUP BY u.id;`,
		id.String(),
	)
	if err != nil {
		return
	}
	if !rows.Next() {
		err = apiErrors.NotFoundError{Msg: "User not found"}
		return
	}

	u = UserModel{}
	var roles pgtype.Array[pgtype.Text]
	var connectionsJSON []byte

	rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt, &u.Private, &u.IsVerified, &roles, &connectionsJSON)
	// assign roles
	u.Roles = []string{}
	for _, role := range roles.Elements {
		if role.String != "" {
			u.Roles = append(u.Roles, role.String)
		}
	}
	// assign connections
	json.Unmarshal(connectionsJSON, &u.Connections)

	return
}

func (u *UserModel) Insert() (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(u.Email)
	u.Password = strings.TrimSpace(u.Password)

	// Generate a secure high-entropy verification token (32 bytes = 256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return 0, err
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(24 * time.Hour)
	u.VerificationToken = token
	u.VerificationTokenExpiresAt = &expiresAt
	u.IsVerified = false

	hashedPass := helpers.HashPassword(u.Password)
	err = conn.QueryRow(
		context.Background(),
		"INSERT INTO Users (name, email, password, is_verified, verification_token, verification_token_expires_at) VALUES ($1, $2, $3, FALSE, $4, $5) RETURNING id",
		u.Name, u.Email, hashedPass, u.VerificationToken, u.VerificationTokenExpiresAt,
	).Scan(&u.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return 0, apiErrors.DuplicateError{Msg: "Username or email already in use"}
			}
		}
		return 0, err
	}
	return 1, nil
}

func (u *UserModel) InsertRole(role string) (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	t, err := conn.Exec(context.Background(),
		"INSERT INTO userroles VALUES ($1, $2)",
		u.ID, role,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return 0, apiErrors.DuplicateError{Msg: "Already assigned"}
			case "23503":
				return 0, apiErrors.NotFoundError{Msg: "User or role not found"}
			}
		}
		return 0, err
	}

	return int(t.RowsAffected()), nil
}

func (u *UserModel) RemoveRole(role string) (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	t, err := conn.Exec(context.Background(),
		"DELETE FROM userroles WHERE userid=$1 AND rolename=$2",
		u.ID.String(), role,
	)
	return int(t.RowsAffected()), err
}

func (u *UserModel) Delete() (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	t, err := conn.Exec(context.Background(), "DELETE FROM users WHERE id=$1", u.ID.String())
	return int(t.RowsAffected()), err
}

func (UserModel) GetUserByEmail(email string) (*UserModel, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	email = strings.TrimSpace(email)
	rows, err := conn.Query(context.Background(),
		`SELECT u.id, u.name, u.email, array_remove(array_agg(ur.rolename), NULL) AS roles
		FROM users u
		LEFT JOIN userroles ur ON u.id = ur.userid
		WHERE LOWER(u.email) = LOWER($1)
		GROUP BY u.id`,
		email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, apiErrors.NotFoundError{Msg: "User not found"}
	}

	u := &UserModel{Email: email}
	var roles pgtype.Array[pgtype.Text]
	if err := rows.Scan(&u.ID, &u.Name, &u.Email, &roles); err != nil {
		return nil, err
	}
	u.Roles = []string{}
	for _, role := range roles.Elements {
		if role.String != "" {
			u.Roles = append(u.Roles, role.String)
		}
	}
	return u, nil
}

func (UserModel) GetUserByProvider(provider, providerUserId string) (*UserModel, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(),
		`SELECT u.id, u.name, u.email, array_remove(array_agg(ur.rolename), NULL) AS roles
		FROM users u
		JOIN userconnections uc ON u.id = uc.userid
		LEFT JOIN userroles ur ON u.id = ur.userid
		WHERE uc.provider = $1 AND uc.provideruserid = $2
		GROUP BY u.id`,
		provider, providerUserId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, apiErrors.NotFoundError{Msg: "User not found"}
	}

	u := &UserModel{}
	var roles pgtype.Array[pgtype.Text]
	if err := rows.Scan(&u.ID, &u.Name, &u.Email, &roles); err != nil {
		return nil, err
	}
	u.Roles = []string{}
	for _, role := range roles.Elements {
		if role.String != "" {
			u.Roles = append(u.Roles, role.String)
		}
	}
	return u, nil
}

func (UserModel) CreateFederatedUser(name, email string) (*UserModel, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())

	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)

	// Generate a secure random dummy password hashed with bcrypt to prevent empty/guessable password logins
	randomSecret, err := helpers.GenerateStateToken()
	if err != nil {
		randomSecret = uuid.Must(uuid.NewV4()).String()
	}
	hashedPass := helpers.HashPassword(randomSecret)

	var newID uuid.UUID
	err = conn.QueryRow(
		context.Background(),
		"INSERT INTO Users (name, email, password, is_verified) VALUES ($1, $2, $3, TRUE) RETURNING id",
		name, email, hashedPass,
	).Scan(&newID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, apiErrors.DuplicateError{Msg: "Username or email already in use"}
		}
		return nil, err
	}

	return &UserModel{
		ID:         newID,
		Name:       name,
		Email:      email,
		IsVerified: true,
		Roles:      []string{},
	}, nil
}

func (UserModel) VerifyEmail(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return apiErrors.ValidationError{Msg: "Verification token is required"}
	}

	conn, err := db.ConnectDB()
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	var userID uuid.UUID
	var isVerified bool
	var expiresAt *time.Time

	err = conn.QueryRow(
		context.Background(),
		"SELECT id, is_verified, verification_token_expires_at FROM Users WHERE verification_token = $1",
		token,
	).Scan(&userID, &isVerified, &expiresAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apiErrors.NotFoundError{Msg: "Invalid or expired verification token"}
		}
		return err
	}

	if isVerified {
		return nil
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		return apiErrors.ValidationError{Msg: "Verification token has expired. Please request a new verification token."}
	}

	_, err = conn.Exec(
		context.Background(),
		"UPDATE Users SET is_verified = TRUE, verification_token = NULL, verification_token_expires_at = NULL, updatedat = NOW() WHERE id = $1",
		userID,
	)
	return err
}


