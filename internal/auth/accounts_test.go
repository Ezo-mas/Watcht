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
	member, err := s.CreateAccount(ctx, admin, "member", "member password", false)
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

	second, err := s.CreateAccount(ctx, admin, "second-admin", "long enough", true)
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
		if _, err := s.CreateAccount(ctx, admin, tc.username, tc.password, false); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestOnlyAdminsManageAccounts(t *testing.T) {
	s, admin, _ := setupWithMember(t)
	ctx := context.Background()
	m, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}

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
	if _, err := s.CreateAccount(ctx, admin, "Bob", "long enough", false); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAccounts(ctx, admin)
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

	if err := s.DeleteAccount(ctx, admin, admin.User.ID); !errors.Is(err, ErrSelf) {
		t.Errorf("deleting yourself: %v, want ErrSelf", err)
	}
	if err := s.DeleteAccount(ctx, admin, member.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := s.Authenticate(ctx, memberSession.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("deleted account's session: %v, want ErrNoSession", err)
	}
	if _, err := s.Login(ctx, "member", "member password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("deleted account can still sign in: %v", err)
	}
	if err := s.DeleteAccount(ctx, admin, member.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: %v, want ErrNotFound", err)
	}
	// The username is free again.
	if _, err := s.CreateAccount(ctx, admin, "member", "long enough", false); err != nil {
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

	if err := s.ResetPassword(ctx, admin, member.ID, "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("short password: %v, want ErrInvalidPassword", err)
	}
	if err := s.ResetPassword(ctx, admin, admin.User.ID, "new admin password"); !errors.Is(err, ErrSelf) {
		t.Errorf("resetting your own password: %v, want ErrSelf", err)
	}
	if err := s.ResetPassword(ctx, admin, 9999, "long enough"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: %v, want ErrNotFound", err)
	}

	if err := s.ResetPassword(ctx, admin, member.ID, "reset password"); err != nil {
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
	s, admin, _ := setupWithMember(t)
	ctx := context.Background()
	current, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword(ctx, current, "wrong password", "new password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong current password: %v, want ErrInvalidCredentials", err)
	}
	if err := s.ChangePassword(ctx, current, "member password", "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("short new password: %v, want ErrInvalidPassword", err)
	}
	if _, err := s.Authenticate(ctx, other.Token); err != nil {
		t.Fatalf("a rejected change ended sessions: %v", err)
	}

	if err := s.ChangePassword(ctx, current, "member password", "new password"); err != nil {
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
	err = s.ChangePassword(ctx, admin, "anything", "new password")
	if !errors.Is(err, ErrInvalidCredentials) || verifies != 1 {
		t.Errorf("account without a password: err = %v after %d verifications; want ErrInvalidCredentials after 1", err, verifies)
	}
}

// Races: each test uses the verify hook to run a competing action at the
// worst moment, after a check but before the write. The write transaction
// must notice.

// onFirstVerify runs fn the first time a password is verified.
func onFirstVerify(s *Service, fn func()) {
	done := false
	s.hasher.testHookVerify = func() {
		if !done {
			done = true
			fn()
		}
	}
}

func TestAdminsCantDeleteEachOther(t *testing.T) {
	s, a, _ := setupWithMember(t)
	ctx := context.Background()
	if _, err := s.CreateAccount(ctx, a, "admin-b", "long enough", true); err != nil {
		t.Fatal(err)
	}
	b, err := s.Login(ctx, "admin-b", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	// Both requests were authenticated before either delete ran.
	if err := s.DeleteAccount(ctx, a, b.User.ID); err != nil {
		t.Fatalf("A deletes B: %v", err)
	}
	if err := s.DeleteAccount(ctx, b, a.User.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("B (already deleted) deletes A: %v, want ErrForbidden", err)
	}
	if _, err := s.Authenticate(ctx, a.Token); err != nil {
		t.Errorf("A was deleted: %v", err)
	}
}

// A request from a deleted admin fails even if a new admin account got
// the same ID (SQLite reuses the highest freed ID).
func TestDeletedAdminCantActThroughAReusedID(t *testing.T) {
	s, a, _ := setupWithMember(t)
	ctx := context.Background()
	if _, err := s.CreateAccount(ctx, a, "admin-b", "long enough", true); err != nil {
		t.Fatal(err)
	}
	b, err := s.Login(ctx, "admin-b", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAccount(ctx, a, b.User.ID); err != nil {
		t.Fatal(err)
	}
	newcomer, err := s.CreateAccount(ctx, a, "newcomer", "long enough", true)
	if err != nil {
		t.Fatal(err)
	}
	if newcomer.ID != b.User.ID {
		t.Fatalf("newcomer got ID %d, not the freed %d; the test needs SQLite to reuse it", newcomer.ID, b.User.ID)
	}
	if _, err := s.CreateAccount(ctx, b, "sneaky", "long enough", true); !errors.Is(err, ErrForbidden) {
		t.Errorf("CreateAccount by the deleted admin: %v, want ErrForbidden", err)
	}
	if err := s.ResetPassword(ctx, b, a.User.ID, "taken over!"); !errors.Is(err, ErrForbidden) {
		t.Errorf("ResetPassword by the deleted admin: %v, want ErrForbidden", err)
	}
	if err := s.DeleteAccount(ctx, b, a.User.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("DeleteAccount by the deleted admin: %v, want ErrForbidden", err)
	}
}

func TestDemotedAdminCantAct(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	a, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, "UPDATE users SET is_admin = 0"); err != nil {
		t.Fatal(err)
	}
	// a.User.IsAdmin is still true in the stale snapshot.
	if _, err := s.CreateAccount(ctx, a, "sneaky", "long enough", true); !errors.Is(err, ErrForbidden) {
		t.Errorf("CreateAccount after demotion: %v, want ErrForbidden", err)
	}
}

// A login that verified the old password must not get a session once a
// reset has committed.
func TestLoginRacingAReset(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	onFirstVerify(s, func() {
		if err := s.ResetPassword(ctx, admin, member.ID, "reset password"); err != nil {
			t.Errorf("reset: %v", err)
		}
	})
	if _, err := s.Login(ctx, "member", "member password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("login with the old password during a reset: %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Login(ctx, "member", "reset password"); err != nil {
		t.Errorf("the reset password doesn't work: %v", err)
	}
}

// Same for an account deleted mid-login whose ID went to a new account.
func TestLoginRacingDeleteAndIDReuse(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	onFirstVerify(s, func() {
		if err := s.DeleteAccount(ctx, admin, member.ID); err != nil {
			t.Errorf("delete: %v", err)
		}
		n, err := s.CreateAccount(ctx, admin, "newcomer", "newcomer password", false)
		if err != nil || n.ID != member.ID {
			t.Errorf("newcomer = %+v, %v; want the reused ID %d", n, err, member.ID)
		}
	})
	sess, err := s.Login(ctx, "member", "member password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("login of a deleted account: %+v, %v; want ErrInvalidCredentials", sess.User, err)
	}
}

// Two changes from two sessions of one account: the one that commits
// first wins, and the other fails instead of overwriting it.
func TestConcurrentPasswordChanges(t *testing.T) {
	s, _, _ := setupWithMember(t)
	ctx := context.Background()
	x, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	y, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	onFirstVerify(s, func() {
		if err := s.ChangePassword(ctx, y, "member password", "y's password"); err != nil {
			t.Errorf("y's change: %v", err)
		}
	})
	err = s.ChangePassword(ctx, x, "member password", "x's password")
	if !errors.Is(err, ErrNoSession) && !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("x's change after y's committed: %v, want it refused", err)
	}
	if _, err := s.Login(ctx, "member", "y's password"); err != nil {
		t.Errorf("y's password was overwritten: %v", err)
	}
	if _, err := s.Authenticate(ctx, y.Token); err != nil {
		t.Errorf("y's session ended: %v", err)
	}
}

// The compare-and-swap on its own: with the session still valid, a change
// based on a password that has since changed is refused.
func TestChangeRefusedIfPasswordChangedMeanwhile(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	a, err := s.Setup(ctx, "admin", "first password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := newHasher(cheap, 1).hash(ctx, "set elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	onFirstVerify(s, func() {
		if _, err := d.Write.ExecContext(ctx, "UPDATE users SET password_hash = ?", other); err != nil {
			t.Error(err)
		}
	})
	if err := s.ChangePassword(ctx, a, "first password", "new password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("change after the password changed: %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Login(ctx, "admin", "set elsewhere"); err != nil {
		t.Errorf("the other change was overwritten: %v", err)
	}
}

// A member's change racing an admin's reset: the reset wins.
func TestChangeRacingAReset(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	m, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	onFirstVerify(s, func() {
		if err := s.ResetPassword(ctx, admin, member.ID, "reset password"); err != nil {
			t.Errorf("reset: %v", err)
		}
	})
	if err := s.ChangePassword(ctx, m, "member password", "member's choice"); !errors.Is(err, ErrNoSession) {
		t.Errorf("change during a reset: %v, want ErrNoSession (the reset ended the session)", err)
	}
	if _, err := s.Login(ctx, "member", "reset password"); err != nil {
		t.Errorf("the reset was overwritten: %v", err)
	}
}
