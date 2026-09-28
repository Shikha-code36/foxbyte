// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"errors"
	"testing"
)

func TestChangeRequestLifecycle(t *testing.T) {
	s := testStore(t)
	u, err := s.CreateUser("asker@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	approver, err := s.CreateUser("approver@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateChangeRequest(u.ID, "dev", "main", 12, `[{"id":13}]`)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.Status != RequestOpen || c.Source != "dev" || c.Target != "main" {
		t.Fatalf("unexpected request: %+v", c)
	}
	if c.CreatedBy != "asker@example.com" || c.ForkAfterID != 12 {
		t.Errorf("who asked, or where they forked, is wrong: %+v", c)
	}

	// Open requests are listable by state and by target.
	open, err := s.ChangeRequests(RequestOpen, "main")
	if err != nil || len(open) != 1 {
		t.Fatalf("listing open requests for main gave %d, %v", len(open), err)
	}
	if none, _ := s.ChangeRequests(RequestOpen, "other"); len(none) != 0 {
		t.Errorf("a request for main was listed under another target")
	}

	// A decision is made once: a second one must not apply the statements again.
	got, err := s.ClaimChangeRequest(c.ID, approver.ID, RequestApproved, "looks right")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != RequestApproved || got.DecidedBy != "approver@example.com" || got.Note != "looks right" {
		t.Errorf("the decision was not recorded: %+v", got)
	}
	if _, err := s.ClaimChangeRequest(c.ID, approver.ID, RequestApproved, ""); !errors.Is(err, ErrRequestDecided) {
		t.Errorf("approving twice gave %v, want ErrRequestDecided", err)
	}
	if _, err := s.ClaimChangeRequest(c.ID, approver.ID, RequestRejected, ""); !errors.Is(err, ErrRequestDecided) {
		t.Errorf("rejecting an approved request gave %v, want ErrRequestDecided", err)
	}

	// Applying records how much went in; a failure records why.
	if err := s.FinishChangeRequest(c.ID, 3, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ChangeRequest(c.ID); got.Applied != 3 || got.Status != RequestApproved {
		t.Errorf("after applying: %+v", got)
	}
	if err := s.FinishChangeRequest(c.ID, 0, "the target went away"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ChangeRequest(c.ID); got.Status != RequestFailed || got.Note == "" {
		t.Errorf("a failure should be recorded: %+v", got)
	}
}

func TestChangeRequestUnknownID(t *testing.T) {
	s := testStore(t)
	if _, err := s.ChangeRequest(999); !errors.Is(err, ErrNoSuchRequest) {
		t.Errorf("an unknown id gave %v, want ErrNoSuchRequest", err)
	}
	if _, err := s.ClaimChangeRequest(999, 1, RequestApproved, ""); !errors.Is(err, ErrNoSuchRequest) {
		t.Errorf("deciding an unknown id gave %v, want ErrNoSuchRequest", err)
	}
}

// The CLI has no account: whoever has a shell on the machine is the operator, and
// the record says so rather than naming a deleted user.
func TestChangeRequestFromTheCLI(t *testing.T) {
	s := testStore(t)
	c, err := s.CreateChangeRequest(CLIUser, "dev", "main", 0, "[]")
	if err != nil {
		t.Fatal(err)
	}
	if c.CreatedBy != "the local CLI" {
		t.Errorf("created_by = %q, want the local CLI", c.CreatedBy)
	}
	got, err := s.ClaimChangeRequest(c.ID, CLIUser, RequestRejected, "not now")
	if err != nil {
		t.Fatal(err)
	}
	if got.DecidedBy != "the local CLI" {
		t.Errorf("decided_by = %q, want the local CLI", got.DecidedBy)
	}
}

// A deleted branch must not leave open requests pointing at nothing.
func TestForgetBranchRequests(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreateChangeRequest(CLIUser, "gone", "main", 0, "[]"); err != nil {
		t.Fatal(err)
	}
	decided, err := s.CreateChangeRequest(CLIUser, "gone", "main", 0, "[]")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimChangeRequest(decided.ID, CLIUser, RequestApproved, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetBranchRequests("gone"); err != nil {
		t.Fatal(err)
	}
	left, err := s.ChangeRequests("", "")
	if err != nil {
		t.Fatal(err)
	}
	// The decided one is history and stays; the open one is gone.
	if len(left) != 1 || left[0].ID != decided.ID {
		t.Errorf("after forgetting the branch: %+v", left)
	}
}

// Deleting a branch forgets its owner and any request still waiting on it, in one
// place, so the CLI and the API cannot disagree about what deletion means.
func TestForgetBranchAlsoForgetsRequests(t *testing.T) {
	s := testStore(t)
	u, err := s.CreateUser("owner@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBranchOwner("doomed", u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChangeRequest(u.ID, "doomed", "main", 0, "[]"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetBranch("doomed"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.BranchOwner("doomed"); ok {
		t.Error("the owner outlived the branch")
	}
	left, err := s.ChangeRequests(RequestOpen, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("an open request outlived its branch: %+v", left)
	}
}
