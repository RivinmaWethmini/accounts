package models

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/sliitmozilla/accounts/db"
)

type ConsentedRedirectModel struct {
	UserID string `json:"userId"`
	Host   string `json:"host"`
}

func (c ConsentedRedirectModel) Exists() (bool, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())

	var exists bool
	err = conn.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM consented_redirects WHERE user_id = $1 AND host = $2)",
		c.UserID, c.Host,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (c ConsentedRedirectModel) Insert() (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	t, err := conn.Exec(context.Background(),
		"INSERT INTO consented_redirects (user_id, host) VALUES ($1, $2) ON CONFLICT (user_id, host) DO NOTHING",
		c.UserID, c.Host,
	)
	if err != nil {
		return 0, err
	}
	return int(t.RowsAffected()), nil
}

func (ConsentedRedirectModel) SelectAllForUser(userID string) (hosts []string, err error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(),
		"SELECT host FROM consented_redirects WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	hosts, err = pgx.CollectRows(rows, pgx.RowTo[string])
	return
}

func (c ConsentedRedirectModel) Delete() (int, error) {
	conn, err := db.ConnectDB()
	if err != nil {
		return 0, err
	}
	defer conn.Close(context.Background())

	t, err := conn.Exec(context.Background(),
		"DELETE FROM consented_redirects WHERE user_id = $1 AND host = $2",
		c.UserID, c.Host,
	)
	if err != nil {
		return 0, err
	}
	return int(t.RowsAffected()), nil
}
