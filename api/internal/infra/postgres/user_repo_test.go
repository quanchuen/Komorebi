package postgres_test

import (
	"testing"

	"komorebi/internal/domain/community"
	"komorebi/internal/infra/postgres"
)

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, err := community.NewUser("Yuki Tanaka", uniqueEmail(t))
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	if err := repo.Create(u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	got, err := repo.GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("DisplayName: want %q, got %q", u.DisplayName, got.DisplayName)
	}
	if got.Email != u.Email {
		t.Errorf("Email: want %q, got %q", u.Email, got.Email)
	}
}

func TestUserRepo_GetByEmail(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	email := uniqueEmail(t)
	u, _ := community.NewUser("Kenji Sato", email)
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	got, err := repo.GetByEmail(email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID: want %q, got %q", u.ID, got.ID)
	}
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	_, err := repo.GetByEmail("nobody@nowhere.example")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
}

func TestUserRepo_DuplicateEmail(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	email := uniqueEmail(t)
	u1, _ := community.NewUser("Alice", email)
	u2, _ := community.NewUser("Bob", email)
	_ = repo.Create(u1)
	t.Cleanup(func() { _ = repo.Delete(u1.ID) })

	err := repo.Create(u2)
	if err == nil {
		t.Fatal("expected duplicate email error")
	}
}

func TestUserRepo_PasswordHash(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, _ := community.NewUser("Hana", uniqueEmail(t))
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	const hash = "$2a$10$testhashvalue"
	if err := repo.SetPasswordHash(u.ID, hash); err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}
	got, err := repo.GetPasswordHash(u.ID)
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	if got != hash {
		t.Errorf("hash: want %q, got %q", hash, got)
	}
}

func TestUserRepo_Update(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewUserRepo(pool)

	u, _ := community.NewUser("Original Name", uniqueEmail(t))
	_ = repo.Create(u)
	t.Cleanup(func() { _ = repo.Delete(u.ID) })

	u.DisplayName = "Updated Name"
	u.AvatarURL = "https://example.com/avatar.png"
	if err := repo.Update(u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(u.ID)
	if got.DisplayName != "Updated Name" {
		t.Errorf("DisplayName after update: %q", got.DisplayName)
	}
	if got.AvatarURL != "https://example.com/avatar.png" {
		t.Errorf("AvatarURL after update: %q", got.AvatarURL)
	}
}

// uniqueEmail generates a test email that won't collide between test runs.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	return "test-" + genUUID() + "@example.com"
}
