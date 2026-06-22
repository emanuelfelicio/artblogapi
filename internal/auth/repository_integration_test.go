package auth

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	dbgen "github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

var (
	testDBPool *pgxpool.Pool
	testRepo   Repository
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env")
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		os.Stderr.WriteString("DATABASE_URL not set, aborting integration tests\n")
		os.Exit(1)
	}

	var err error
	testDBPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		os.Stderr.WriteString("failed to create pgx pool: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer testDBPool.Close()

	sqlDB := stdlib.OpenDB(*testDBPool.Config().ConnConfig)
	defer sqlDB.Close()

	gooseDir := os.Getenv("GOOSE_DIR")
	if err = goose.Up(sqlDB, gooseDir); err != nil {
		os.Stderr.WriteString("goose up failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	queries := dbgen.New(testDBPool)
	testRepo = NewRepository(queries, nil, testDBPool)

	os.Exit(m.Run())
}

// setup registers table truncation as a cleanup step and returns a background context.
func setup(t *testing.T) context.Context {
	t.Helper()
	t.Cleanup(func() {
		_, err := testDBPool.Exec(context.Background(), `TRUNCATE TABLE users, sessions RESTART IDENTITY CASCADE`)
		if err != nil {
			t.Errorf("failed to truncate tables: %v", err)
		}
	})
	return context.Background()
}

func newTestUser() User {
	return User{
		ID:           uuid.New(),
		Username:     "u_" + uuid.NewString()[:8],
		Email:        "u_" + uuid.NewString()[:8] + "@example.com",
		PasswordHash: "hashed",
	}
}

func newTestSession(userID uuid.UUID) Session {
	return Session{
		ID:        uuid.NewString(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
}

func mustCreateUser(t *testing.T, ctx context.Context) User {
	t.Helper()
	u := newTestUser()
	_, err := testRepo.CreateUser(ctx, u)
	if err != nil {
		t.Fatalf("mustCreateUser: %v", err)
	}
	return u
}

func mustSaveSession(t *testing.T, ctx context.Context, userID uuid.UUID) Session {
	t.Helper()
	s := newTestSession(userID)
	if err := testRepo.SaveSession(ctx, s); err != nil {
		t.Fatalf("mustSaveSession: %v", err)
	}
	return s
}

func TestRepository_CreateUser(t *testing.T) {
	t.Run("creates user and returns correct ID", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()

		created, err := testRepo.CreateUser(ctx, u)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if created.ID != u.ID {
			t.Fatalf("ID mismatch: got %v, want %v", created.ID, u.ID)
		}
	})

	t.Run("returns ErrEmailAlreadyExists on duplicate email", func(t *testing.T) {
		ctx := setup(t)
		original := mustCreateUser(t, ctx)

		dup := newTestUser()
		dup.Email = original.Email
		_, err := testRepo.CreateUser(ctx, dup)

		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("want ErrEmailAlreadyExists, got: %v", err)
		}
	})

	t.Run("returns ErrUsernameAlreadyExists on duplicate username", func(t *testing.T) {
		ctx := setup(t)
		original := mustCreateUser(t, ctx)

		dup := newTestUser()
		dup.Username = original.Username
		_, err := testRepo.CreateUser(ctx, dup)

		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Fatalf("want ErrUsernameAlreadyExists, got: %v", err)
		}
	})
}

func TestRepository_CheckEmailAndUsername(t *testing.T) {
	t.Run("returns ErrEmailAlreadyExists when only email conflicts", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)

		err := testRepo.CheckEmailAndUsername(ctx, "unique_"+uuid.NewString()[:8], u.Email)
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("want ErrEmailAlreadyExists, got: %v", err)
		}
	})

	t.Run("returns ErrUsernameAlreadyExists when only username conflicts", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)

		err := testRepo.CheckEmailAndUsername(ctx, u.Username, "unique_"+uuid.NewString()[:8]+"@x.com")
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Fatalf("want ErrUsernameAlreadyExists, got: %v", err)
		}
	})

	t.Run("returns both sentinel errors when email and username conflict", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)

		err := testRepo.CheckEmailAndUsername(ctx, u.Username, u.Email)
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Fatalf("want ErrEmailAlreadyExists in joined error, got: %v", err)
		}
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Fatalf("want ErrUsernameAlreadyExists in joined error, got: %v", err)
		}
	})
}

func TestRepository_FindUserByCredential(t *testing.T) {
	t.Run("finds user by email", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)

		found, err := testRepo.FindUserByCredential(ctx, u.Email)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if found.Email != u.Email {
			t.Fatalf("email mismatch: got %v, want %v", found.Email, u.Email)
		}
	})

	t.Run("returns ErrInvalidCredentials for unknown credential", func(t *testing.T) {
		ctx := setup(t)

		_, err := testRepo.FindUserByCredential(ctx, "nobody@example.com")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("want ErrInvalidCredentials, got: %v", err)
		}
	})
}

func TestRepository_Session(t *testing.T) {
	t.Run("saves and retrieves session by ID", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)
		s := mustSaveSession(t, ctx, u.ID)

		fetched, err := testRepo.FindSessionByID(ctx, s.ID)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if fetched.ID != s.ID {
			t.Fatalf("ID mismatch: got %v, want %v", fetched.ID, s.ID)
		}
	})

	t.Run("persists optional fields (UserAgent, IP, DeviceID)", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)
		s := newTestSession(u.ID)
		s.UserAgent = "Mozilla/5.0"
		s.IP = "192.168.1.1"
		s.DeviceID = "device-abc"

		if err := testRepo.SaveSession(ctx, s); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}
		fetched, err := testRepo.FindSessionByID(ctx, s.ID)
		if err != nil {
			t.Fatalf("FindSessionByID: %v", err)
		}
		if fetched.UserAgent != s.UserAgent {
			t.Errorf("UserAgent: got %v, want %v", fetched.UserAgent, s.UserAgent)
		}
		if fetched.IP != s.IP {
			t.Errorf("IP: got %v, want %v", fetched.IP, s.IP)
		}
		if fetched.DeviceID != s.DeviceID {
			t.Errorf("DeviceID: got %v, want %v", fetched.DeviceID, s.DeviceID)
		}
	})

	t.Run("returns ErrSessionNotFound for unknown ID", func(t *testing.T) {
		ctx := setup(t)

		_, err := testRepo.FindSessionByID(ctx, uuid.NewString())
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("want ErrSessionNotFound, got: %v", err)
		}
	})

	t.Run("FindSessionByIDForUpdate returns ErrSessionNotFound for unknown ID", func(t *testing.T) {
		ctx := setup(t)

		err := testRepo.WithTransaction(ctx, func(r Repository) error {
			_, err := r.FindSessionByIDForUpdate(ctx, uuid.NewString())
			return err
		})
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("want ErrSessionNotFound, got: %v", err)
		}
	})

	t.Run("revokes session inside transaction", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)
		s := mustSaveSession(t, ctx, u.ID)

		err := testRepo.WithTransaction(ctx, func(r Repository) error {
			locked, err := r.FindSessionByIDForUpdate(ctx, s.ID)
			if err != nil {
				return err
			}
			if locked.ID != s.ID {
				t.Errorf("lock returned wrong session: got %v, want %v", locked.ID, s.ID)
			}
			return r.RevokeSessionByID(ctx, s.ID)
		})
		if err != nil {
			t.Fatalf("WithTransaction: %v", err)
		}

		revoked, err := testRepo.FindSessionByID(ctx, s.ID)
		if err != nil {
			t.Fatalf("FindSessionByID after revoke: %v", err)
		}
		if !revoked.Revoked {
			t.Fatal("session not revoked after transaction")
		}
	})

	t.Run("revokes all sessions for a user", func(t *testing.T) {
		ctx := setup(t)
		u := mustCreateUser(t, ctx)

		var ids []string
		for range 3 {
			s := mustSaveSession(t, ctx, u.ID)
			ids = append(ids, s.ID)
		}

		if err := testRepo.RevokeAllSessionsByUserID(ctx, u.ID); err != nil {
			t.Fatalf("RevokeAllSessionsByUserID: %v", err)
		}

		for _, id := range ids {
			s, err := testRepo.FindSessionByID(ctx, id)
			if err != nil {
				t.Fatalf("FindSessionByID(%v): %v", id, err)
			}
			if !s.Revoked {
				t.Errorf("session %v not revoked", id)
			}
		}
	})
}
