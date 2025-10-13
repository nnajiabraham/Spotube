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

func (r *SQLiteTokenRepository) UpsertToken(provider string, token Token) error {
	now := time.Now().Unix()
	id := uuid.NewString()

	_, err := table.OAuthTokens.
		INSERT(
			table.OAuthTokens.ID,
			table.OAuthTokens.Provider,
			table.OAuthTokens.AccessToken,
			table.OAuthTokens.RefreshToken,
			table.OAuthTokens.Expiry,
			table.OAuthTokens.Scopes,
			table.OAuthTokens.Created,
			table.OAuthTokens.Updated,
		).
		VALUES(
			id,
			provider,
			token.AccessToken,
			token.RefreshToken,
			token.Expiry,
			token.Scopes,
			now,
			now,
		).
		ON_CONFLICT(table.OAuthTokens.Provider).
		DO_UPDATE(sqlite.SET(
			table.OAuthTokens.AccessToken.SET(sqlite.String(token.AccessToken.String)),
			table.OAuthTokens.RefreshToken.SET(sqlite.String(token.RefreshToken.String)),
			table.OAuthTokens.Expiry.SET(sqlite.Int(token.Expiry.Int64)),
			table.OAuthTokens.Scopes.SET(sqlite.String(token.Scopes.String)),
			table.OAuthTokens.Updated.SET(sqlite.Int(now)),
		)).
		Exec(r.db)

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
