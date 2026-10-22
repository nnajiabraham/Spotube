package auth

import (
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/google/uuid"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// SQLiteTokenRepository implements TokenRepository using Jet and SQLite.
type SQLiteTokenRepository struct {
	db *sql.DB
}

func NewSQLiteTokenRepository(db *sql.DB) TokenRepository {
	return &SQLiteTokenRepository{db: db}
}

func (r *SQLiteTokenRepository) GetToken(provider string) (*Token, error) {
	var tokens []model.OAuthTokens
	err := table.OAuthTokens.
		SELECT(table.OAuthTokens.AllColumns).
		WHERE(table.OAuthTokens.Provider.EQ(sqlite.String(provider))).
		Query(r.db, &tokens)

	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	token := tokens[0]

	return &Token{
		AccessToken:  nullableStringFromPtr(token.AccessToken),
		RefreshToken: nullableStringFromPtr(token.RefreshToken),
		Expiry:       nullableInt64FromPtr(token.Expiry),
		Scopes:       nullableStringFromPtr(token.Scopes),
	}, nil
}

// UpsertToken stores provider tokens. An empty refresh token keeps the stored one,
// because providers only return a refresh token on some exchanges.
func (r *SQLiteTokenRepository) UpsertToken(provider string, token Token) error {
	now := time.Now().Unix()
	_, err := r.db.Exec(
		`INSERT INTO oauth_tokens (id, provider, access_token, refresh_token, expiry, scopes, created, updated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(provider) DO UPDATE SET
		   access_token = excluded.access_token,
		   refresh_token = COALESCE(NULLIF(excluded.refresh_token, ''), oauth_tokens.refresh_token),
		   expiry = excluded.expiry,
		   scopes = COALESCE(NULLIF(excluded.scopes, ''), oauth_tokens.scopes),
		   updated = excluded.updated`,
		uuid.NewString(), provider, token.AccessToken, token.RefreshToken, token.Expiry, token.Scopes, now, now,
	)
	return err
}

func nullableStringFromPtr(ptr *string) sql.NullString {
	if ptr == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *ptr, Valid: true}
}

func nullableInt64FromPtr(ptr *int32) sql.NullInt64 {
	if ptr == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*ptr), Valid: true}
}
