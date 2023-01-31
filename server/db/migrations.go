package db

import (
	"github.com/juju/errors"
)

func (db *DB) Reset() error {
	_, err := db.db.Exec("DROP TABLE IF EXISTS jobs")
	if err != nil {
		return errors.Trace(err)
	}

	_, err = db.db.Exec("DROP TABLE IF EXISTS jobs_archive")
	if err != nil {
		return errors.Trace(err)
	}

	_, err = db.db.Exec("DROP TYPE IF EXISTS archive_reason")
	if err != nil {
		return errors.Trace(err)
	}

	err = db.MigrateUp()
	return errors.Trace(err)
}

func (db *DB) MigrateUp() error {
	_, err := db.db.Exec(`
CREATE TABLE IF NOT EXISTS jobs
(
  id serial PRIMARY KEY,
  uuid uuid UNIQUE NOT NULL,
  created timestamp with time zone NOT NULL DEFAULT now(),
  running boolean NOT NULL DEFAULT false,
  settings jsonb NOT NULL,
	start_time timestamp with time zone,
	end_time timestamp with time zone
);

CREATE TYPE archive_reason AS ENUM ('done', 'cancelled', 'error');

CREATE TABLE IF NOT EXISTS jobs_archive
(
  id bigserial PRIMARY KEY,
  uuid uuid NOT NULL,
  created timestamp with time zone NOT NULL,
  settings jsonb NOT NULL,
	start_time timestamp with time zone,
  end_time timestamp with time zone,
	archive_reason archive_reason NOT NULL,
	job_output text
);
`)

	return errors.Trace(err)
}
