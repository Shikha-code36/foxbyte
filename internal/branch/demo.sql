-- SPDX-License-Identifier: AGPL-3.0-or-later
--
-- Sample data for a first start. A brand-new install used to open the console on
-- an empty database: nothing to select, nothing in the Blackbox, and no way to
-- see what the product does without first inventing a schema. These three tables
-- give a new user something to query, alter and branch in the first minute, and
-- the CREATE TABLEs themselves land in the Blackbox, so the record has content
-- the moment the page loads.
--
-- Seeded into main once (a marker in the state directory), and into any branch on
-- demand with `fox demo seed <branch>`. Set FOX_NO_DEMO=1 to start empty.
-- Everything here is IF NOT EXISTS / ON CONFLICT, so a re-run changes nothing.

-- Who did this, in the Blackbox. Without these the first rows a new user sees
-- are attributed to nobody; with them the record reads "fox demo", which is the
-- truth and also shows what the actor and tool columns are for.
SET bb.actor = 'fox demo';
SET application_name = 'fox demo seed';

CREATE TABLE IF NOT EXISTS users (
	id      serial PRIMARY KEY,
	email   text NOT NULL UNIQUE,
	name    text NOT NULL,
	created timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
	id       serial PRIMARY KEY,
	owner_id integer NOT NULL REFERENCES users(id),
	name     text NOT NULL,
	created  timestamptz NOT NULL DEFAULT now()
);

-- The foreign key is the point of this table: it is what makes a branch, a
-- rewind or an impact analysis show something worth looking at.
CREATE TABLE IF NOT EXISTS events (
	id         bigserial PRIMARY KEY,
	project_id integer NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	kind       text NOT NULL,
	at         timestamptz NOT NULL DEFAULT now(),
	payload    jsonb NOT NULL DEFAULT '{}'::jsonb
);

INSERT INTO users (id, email, name) VALUES
	(1, 'ada@example.com',     'Ada Lovelace'),
	(2, 'grace@example.com',   'Grace Hopper'),
	(3, 'alan@example.com',    'Alan Turing'),
	(4, 'katherine@example.com', 'Katherine Johnson'),
	(5, 'edsger@example.com',  'Edsger Dijkstra')
ON CONFLICT (id) DO NOTHING;

INSERT INTO projects (id, owner_id, name) VALUES
	(1, 1, 'analytics'),
	(2, 1, 'billing'),
	(3, 2, 'search'),
	(4, 3, 'onboarding'),
	(5, 4, 'telemetry')
ON CONFLICT (id) DO NOTHING;

INSERT INTO events (id, project_id, kind, payload) VALUES
	(1, 1, 'deploy',  '{"version": "1.4.0"}'),
	(2, 1, 'rollback','{"version": "1.3.9", "reason": "latency"}'),
	(3, 2, 'invoice', '{"amount": 4200, "currency": "USD"}'),
	(4, 3, 'reindex', '{"documents": 18422}'),
	(5, 5, 'alert',   '{"metric": "p99", "threshold_ms": 250}')
ON CONFLICT (id) DO NOTHING;

-- The serial sequences start at 1, so an INSERT that leaves id to the default
-- would collide with the rows above until the sequence catches up.
SELECT setval(pg_get_serial_sequence('users',    'id'), (SELECT max(id) FROM users));
SELECT setval(pg_get_serial_sequence('projects', 'id'), (SELECT max(id) FROM projects));
SELECT setval(pg_get_serial_sequence('events',   'id'), (SELECT max(id) FROM events));
