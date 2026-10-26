package auth

import (
	"context"
	"errors"
	"testing"
)

// setupWithMember returns a service with an admin (signed in) and a
// regular member.
func setupWithMember(t *testing.T) (*Service, Session, Account) {
	t.Helper()
	s, _, _ := newService(t)
	ctx := context.Background()
	admin, err := s.Setup(ctx, "admin", "admin password")
	if err != nil {
		t.Fatal(err)
	}
	member, err := s.CreateAccount(ctx, admin.User, "member", "member password", false)
	if err != nil {
		t.Fatal(err)
	}
	return s, admin, member
}

func TestCreateAccount(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()

	if member.Username != "member" || member.IsAdmin || member.ID == 0 || member.CreatedAt.IsZero() {
		t.Errorf("created account = %+v", member)
	}
	if _, err := s.Login(ctx, "member", "member password"); err != nil {
		t.Errorf("the new account can't sign in: %v", err)
	}

	second, err := s.CreateAccount(ctx, admin.User, "second-admin", "long enough", true)
	if err != nil || !second.IsAdmin {
		t.Errorf("creating an admin = %+v, %v", second, err)
	}

	for _, tc := range []struct {
		name               string
		username, password string
		want               error
	}{
		{"taken, other case", "MEMBER", "long enough", ErrUsernameTaken},
		{"bad username", "two words", "long enough", ErrInvalidUsername},
		{"short password", "newbie", "short", ErrInvalidPassword},
	} {
		if _, err := s.CreateAccount(ctx, admin.User, tc.username, tc.password, false); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestOnlyAdminsManageAccounts(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	m := member.User

	if _, err := s.ListAccounts(ctx, m); !errors.Is(err, ErrForbidden) {
		t.Errorf("ListAccounts as member: %v, want ErrForbidden", err)
	}
	if _, err := s.CreateAccount(ctx, m, "sneaky", "long enough", true); !errors.Is(err, ErrForbidden) {
		t.Errorf("CreateAccount as member: %v, want ErrForbidden", err)
	}
	if err := s.DeleteAccount(ctx, m, admin.User.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("DeleteAccount as member: %v, want ErrForbidden", err)
	}
	if err := s.ResetPassword(ctx, m, admin.User.ID, "taken over!"); !errors.Is(err, ErrForbidden) {
		t.Errorf("ResetPassword as member: %v, want ErrForbidden", err)
	}
	if _, err := s.Login(ctx, "admin", "admin password"); err != nil {
		t.Errorf("admin's password changed by a member's request: %v", err)
	}
}

func TestListAccounts(t *testing.T) {
	s, admin, _ := setupWithMember(t)
	ctx := context.Background()
	if _, err := s.CreateAccount(ctx, admin.User, "Bob", "long enough", false); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAccounts(ctx, admin.User)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range list {
		names = append(names, a.Username)
	}
	// Ordered by username, ignoring case.
	if want := []string{"admin", "Bob", "member"}; len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Errorf("accounts = %v, want %v", names, want)
	}
	if !list[0].IsAdmin || list[2].IsAdmin {
		t.Errorf("admin flags wrong: %+v", list)
	}
}

func TestDeleteAccount(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	memberSession, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteAccount(ctx, admin.User, admin.User.ID); !errors.Is(err, ErrSelf) {
		t.Errorf("deleting yourself: %v, want ErrSelf", err)
	}
	if err := s.DeleteAccount(ctx, admin.User, member.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := s.Authenticate(ctx, memberSession.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("deleted account's session: %v, want ErrNoSession", err)
	}
	if _, err := s.Login(ctx, "member", "member password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("deleted account can still sign in: %v", err)
	}
	if err := s.DeleteAccount(ctx, admin.User, member.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: %v, want ErrNotFound", err)
	}
	// The username is free again.
	if _, err := s.CreateAccount(ctx, admin.User, "member", "long enough", false); err != nil {
		t.Errorf("reusing a deleted username: %v", err)
	}
}

func TestResetPassword(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	a, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ResetPassword(ctx, admin.User, member.ID, "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("short password: %v, want ErrInvalidPassword", err)
	}
	if err := s.ResetPassword(ctx, admin.User, admin.User.ID, "new admin password"); !errors.Is(err, ErrSelf) {
		t.Errorf("resetting your own password: %v, want ErrSelf", err)
	}
	if err := s.ResetPassword(ctx, admin.User, 9999, "long enough"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: %v, want ErrNotFound", err)
	}

	if err := s.ResetPassword(ctx, admin.User, member.ID, "reset password"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := s.Login(ctx, "member", "member password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password still works: %v", err)
	}
	if _, err := s.Login(ctx, "member", "reset password"); err != nil {
		t.Errorf("new password doesn't work: %v", err)
	}
	// All of the member's earlier sessions end; the admin's don't.
	for name, tok := range map[string]string{"a": a.Token, "b": b.Token} {
		if _, err := s.Authenticate(ctx, tok); !errors.Is(err, ErrNoSession) {
			t.Errorf("member session %s after reset: %v, want ErrNoSession", name, err)
		}
	}
	if _, err := s.Authenticate(ctx, admin.Token); err != nil {
		t.Errorf("admin's session ended by resetting someone else's password: %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	current, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword(ctx, member.User, current.ID, "wrong password", "new password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong current password: %v, want ErrInvalidCredentials", err)
	}
	if err := s.ChangePassword(ctx, member.User, current.ID, "member password", "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("short new password: %v, want ErrInvalidPassword", err)
	}
	if _, err := s.Authenticate(ctx, other.Token); err != nil {
		t.Fatalf("a rejected change ended sessions: %v", err)
	}

	if err := s.ChangePassword(ctx, member.User, current.ID, "member password", "new password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := s.Login(ctx, "member", "new password"); err != nil {
		t.Errorf("new password doesn't work: %v", err)
	}
	// The session that made the change stays; the others end.
	if _, err := s.Authenticate(ctx, current.Token); err != nil {
		t.Errorf("the current session ended: %v", err)
	}
	if _, err := s.Authenticate(ctx, other.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("another session survived the change: %v", err)
	}
	if _, err := s.Authenticate(ctx, admin.Token); err != nil {
		t.Errorf("another user's session ended: %v", err)
	}
}

// Like Login, a password check must cost the same whether or not the
// account has a password.
func TestChangePasswordAlwaysHashes(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	admin, err := s.Setup(ctx, "admin", "admin password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, "UPDATE users SET password_hash = NULL"); err != nil {
		t.Fatal(err)
	}
	var verifies int
	s.hasher.testHookVerify = func() { verifies++ }
	err = s.ChangePassword(ctx, admin.User, admin.ID, "anything", "new password")
	if !errors.Is(err, ErrInvalidCredentials) || verifies != 1 {
		t.Errorf("account without a password: err = %v after %d verifications; want ErrInvalidCredentials after 1", err, verifies)
	}
}
