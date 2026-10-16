package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uptivra/internal/store"
	"uptivra/internal/testutil"
)

func TestAdminOwnershipAndSessionIsolation(t *testing.T) {
	s := testutil.DB(t)
	ctx := context.Background()
	password := "test-password-very-long"
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for _, email := range []string{"owner@example.com", "race@example.com"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.Bootstrap(ctx, email, password) == nil {
				mu.Lock()
				created++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if created != 1 {
		t.Fatalf("created %d owners", created)
	}
	admins, e := s.Admins(ctx)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := s.Login(ctx, admins[0].Email, password)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.CreateAdmin(ctx, owner, "second@example.com", password); e != nil {
		t.Fatal(e)
	}
	second, e := s.Login(ctx, "SECOND@example.com", password)
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.SessionAdmin(ctx, second)
	if e != nil || a.Owner {
		t.Fatal(a, e)
	}
	if !errors.Is(s.ManageAdmin(ctx, second, 1, false), store.ErrAdminAccess) {
		t.Fatal("admin deleted owner")
	}
	if !errors.Is(s.ManageAdmin(ctx, owner, 1, false), store.ErrAdminAccess) {
		t.Fatal("owner deleted itself")
	}
	if e = s.ChangeAdminPassword(ctx, a.ID, password, "replacement-password-long"); e != nil {
		t.Fatal(e)
	}
	if s.Session(ctx, second) || !s.Session(ctx, owner) {
		t.Fatal("password revoked wrong sessions")
	}
	second, e = s.Login(ctx, "second@example.com", "replacement-password-long")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ManageAdmin(ctx, owner, a.ID, true); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(s.ManageAdmin(ctx, owner, a.ID, false), store.ErrAdminAccess) {
		t.Fatal("former owner retained privilege")
	}
	if e = s.ManageAdmin(ctx, second, 1, false); e != nil {
		t.Fatal(e)
	}
	if s.Session(ctx, owner) {
		t.Fatal("deleted admin session survives")
	}
	if e = s.Bootstrap(ctx, "new@example.com", password); e == nil {
		t.Fatal("setup reopened")
	}
}
