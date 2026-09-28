// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Change requests: a branch's schema changes, offered for review before they are
// applied to another branch.
//
// The store keeps the record — who asked, for what, who decided, and the snapshot
// of statements as they were when the request was made — and knows nothing about
// what the statements mean. The reading and applying live in internal/branch,
// beside the Blackbox they come from. A snapshot rather than a live read is the
// point: a reviewer approves what they were shown, not whatever the branch looks
// like by the time they click.

// Change request states.
const (
	RequestOpen     = "open"
	RequestApproved = "approved"
	RequestRejected = "rejected"
	RequestFailed   = "failed" // approved, but applying it did not finish
)

// ErrNoSuchRequest is returned for an id that is not there.
var ErrNoSuchRequest = errors.New("no such change request")

// ErrRequestDecided is returned when a request has already been approved or
// rejected: a decision is made once.
var ErrRequestDecided = errors.New("that change request has already been decided")

// ChangeRequest is one request, with the emails resolved for display.
type ChangeRequest struct {
	ID          int64  `json:"id"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	CreatedBy   string `json:"created_by"`
	Created     string `json:"created"`
	Status      string `json:"status"`
	ForkAfterID int64  `json:"fork_after_id"`
	Entries     string `json:"entries"` // opaque JSON, written and read by internal/branch
	DecidedBy   string `json:"decided_by,omitempty"`
	Decided     string `json:"decided,omitempty"`
	Note        string `json:"note,omitempty"`
	Applied     int    `json:"applied"`
}

// CreateChangeRequest records a new request and returns it.
func (s *Store) CreateChangeRequest(userID int64, source, target string, forkAfterID int64, entriesJSON string) (ChangeRequest, error) {
	if strings.TrimSpace(source) == "" || strings.TrimSpace(target) == "" {
		return ChangeRequest{}, errors.New("a change request needs a source and a target branch")
	}
	if entriesJSON == "" {
		entriesJSON = "[]"
	}
	res, err := s.db.Exec(`INSERT INTO change_requests
		(source, target, created_by, created, status, fork_after_id, entries)
		VALUES (?,?,?,?,?,?,?)`,
		source, target, userID, time.Now().Unix(), RequestOpen, forkAfterID, entriesJSON)
	if err != nil {
		return ChangeRequest{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ChangeRequest{}, err
	}
	return s.ChangeRequest(id)
}

// CLIUser is the user id the CLI records for itself. Whoever has a shell on the
// machine is the operator, and naming that plainly beats inventing an account.
const CLIUser int64 = 0

const cliLabel = "the local CLI"

const requestColumns = `r.id, r.source, r.target,
	CASE WHEN r.created_by = 0 THEN '` + cliLabel + `' ELSE coalesce(u.email,'(deleted account)') END,
	r.created, r.status, r.fork_after_id, r.entries,
	CASE WHEN r.decided_by IS NULL THEN '' WHEN r.decided_by = 0 THEN '` + cliLabel + `'
	     ELSE coalesce(d.email,'(deleted account)') END,
	coalesce(r.decided,0), r.note, r.applied`

func scanRequest(row interface{ Scan(...any) error }) (ChangeRequest, error) {
	var c ChangeRequest
	var created, decided int64
	if err := row.Scan(&c.ID, &c.Source, &c.Target, &c.CreatedBy, &created, &c.Status,
		&c.ForkAfterID, &c.Entries, &c.DecidedBy, &decided, &c.Note, &c.Applied); err != nil {
		return ChangeRequest{}, err
	}
	c.Created = time.Unix(created, 0).UTC().Format(time.RFC3339)
	if decided > 0 {
		c.Decided = time.Unix(decided, 0).UTC().Format(time.RFC3339)
	}
	return c, nil
}

// ChangeRequest is one request by id.
func (s *Store) ChangeRequest(id int64) (ChangeRequest, error) {
	row := s.db.QueryRow(`SELECT `+requestColumns+` FROM change_requests r
		LEFT JOIN users u ON u.id = r.created_by
		LEFT JOIN users d ON d.id = r.decided_by WHERE r.id = ?`, id)
	c, err := scanRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChangeRequest{}, fmt.Errorf("%w: %d", ErrNoSuchRequest, id)
	}
	return c, err
}

// ChangeRequests lists requests, newest first. An empty status means every state;
// an empty target means every branch.
func (s *Store) ChangeRequests(status, target string) ([]ChangeRequest, error) {
	q := `SELECT ` + requestColumns + ` FROM change_requests r
		LEFT JOIN users u ON u.id = r.created_by
		LEFT JOIN users d ON d.id = r.decided_by WHERE 1=1`
	var args []any
	if status != "" {
		q += " AND r.status = ?"
		args = append(args, status)
	}
	if target != "" {
		q += " AND r.target = ?"
		args = append(args, target)
	}
	q += " ORDER BY r.id DESC LIMIT 500"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChangeRequest{}
	for rows.Next() {
		c, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ApprovedRequestsFrom is the approved requests this source has already had
// applied to this target. Promotion needs them: what a branch has already given
// another is not a reason to refuse its next round (internal/branch/promote.go).
func (s *Store) ApprovedRequestsFrom(source, target string) ([]ChangeRequest, error) {
	all, err := s.ChangeRequests(RequestApproved, target)
	if err != nil {
		return nil, err
	}
	out := []ChangeRequest{}
	for _, c := range all {
		if c.Source == source {
			out = append(out, c)
		}
	}
	return out, nil
}

// ClaimChangeRequest moves an open request to a decided state and records who
// decided it. It is the one write that must not race: two approvals of the same
// request would apply its statements twice, so the update is conditional on the
// request still being open and reports ErrRequestDecided when it was not.
func (s *Store) ClaimChangeRequest(id, userID int64, status, note string) (ChangeRequest, error) {
	if status != RequestApproved && status != RequestRejected {
		return ChangeRequest{}, fmt.Errorf("a change request is approved or rejected, not %q", status)
	}
	res, err := s.db.Exec(`UPDATE change_requests SET status=?, decided_by=?, decided=?, note=?
		WHERE id=? AND status=?`, status, userID, time.Now().Unix(), note, id, RequestOpen)
	if err != nil {
		return ChangeRequest{}, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		// Either there is no such request, or somebody else decided it first.
		if _, err := s.ChangeRequest(id); err != nil {
			return ChangeRequest{}, err
		}
		return ChangeRequest{}, fmt.Errorf("%w: %d", ErrRequestDecided, id)
	}
	return s.ChangeRequest(id)
}

// FinishChangeRequest records the outcome of applying an approved request: how
// many statements went in, and the failure when it did not finish.
func (s *Store) FinishChangeRequest(id int64, applied int, failure string) error {
	status := RequestApproved
	note := ""
	if failure != "" {
		status = RequestFailed
		note = failure
	}
	if note == "" {
		_, err := s.db.Exec(`UPDATE change_requests SET status=?, applied=? WHERE id=?`, status, applied, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE change_requests SET status=?, applied=?, note=? WHERE id=?`, status, applied, note, id)
	return err
}

// ForgetBranchRequests removes the requests of a branch that no longer exists, so
// a deleted branch does not leave open requests pointing at nothing.
func (s *Store) ForgetBranchRequests(branch string) error {
	_, err := s.db.Exec(`DELETE FROM change_requests WHERE (source=? OR target=?) AND status=?`,
		branch, branch, RequestOpen)
	return err
}
