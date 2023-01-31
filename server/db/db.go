package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/wellsjo/ai-art/server/job"

	_ "github.com/lib/pq"
)

type DB struct {
	db *sql.DB
}

func Connect(host string, port int, user string, password string, dbname string) (*DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &DB{
		db: db,
	}, nil
}

func (db *DB) AddJob(j job.Job) error {
	result, err := db.db.Exec(
		`INSERT INTO jobs (uuid, created, settings) VALUES ($1, $2, $3)`,
		j.UUID, j.Created, j.Settings,
	)
	if err != nil {
		return errors.Trace(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Trace(err)
	}

	log.Println(rowsAffected, "rows affected")
	return nil
}

// Position is -1 if job is done, 0 if job is running.
func (db *DB) GetJobByUUID(uuid_ uuid.UUID) (job.Job, bool, int, error) {
	j, found, err := db.selectJobByUUID(uuid_)
	if err != nil {
		return job.Job{}, false, 0, errors.Trace(err)
	}
	if found {
		pos, err := db.getJobQueuePosition(j)
		return j, true, pos, errors.Trace(err)
	}

	j, found, err = db.selectJobArchiveByUUID(uuid_)
	if err != nil {
		return job.Job{}, false, 0, errors.Trace(err)
	}
	if !found {
		return job.Job{}, false, 0, nil
	}

	return j, true, -1, nil
}

func (db *DB) getJobQueuePosition(j job.Job) (int, error) {
	rows, err := db.db.Query(`
	SELECT count(*) FROM jobs WHERE created < $1
	`, j.Created)
	if err != nil {
		return 0, errors.Annotate(err, "getJobQueuePosition")
	}
	defer rows.Close()

	rows.Next()
	var position int
	rows.Scan(&position)

	return position, nil
}

func (db *DB) selectJobByUUID(uuid_ uuid.UUID) (job.Job, bool, error) {
	row := db.db.QueryRow(`
	SELECT created, settings, running, start_time, end_time FROM jobs WHERE uuid=$1
	`, uuid_)
	if err := row.Err(); err != nil {
		return job.Job{}, false, errors.Annotate(err, "GetJobByUUID")
	}

	var (
		created   time.Time
		settings  job.Settings
		running   bool
		startTime *time.Time
		endTime   *time.Time
	)
	if err := row.Scan(
		&created, &settings, &running, &startTime, &endTime,
	); err == sql.ErrNoRows {
		return job.Job{}, false, nil
	} else if err != nil {
		return job.Job{}, false, errors.Annotate(err, "GetJobByUUID")
	}

	log.Println("GET JOB", created.UTC())

	return job.Job{
		UUID:      uuid_,
		Created:   created.UTC(),
		Settings:  settings,
		Running:   running,
		StartTime: startTime,
		EndTime:   endTime,
	}, true, nil
}

func (db *DB) selectJobArchiveByUUID(uuid_ uuid.UUID) (job.Job, bool, error) {
	row := db.db.QueryRow(
		`SELECT created, settings, start_time, end_time, archive_reason FROM jobs_archive WHERE uuid=$1`,
		uuid_,
	)
	if err := row.Err(); err != nil {
		return job.Job{}, false, errors.Annotate(err, "GetJobByUUID")
	}

	var (
		created       time.Time
		settings      job.Settings
		startTime     *time.Time
		endTime       *time.Time
		archiveReason job.ArchiveReason
	)
	if err := row.Scan(&created, &settings, &startTime, &endTime, &archiveReason); err == sql.ErrNoRows {
		return job.Job{}, false, nil
	} else if err != nil {
		return job.Job{}, false, errors.Annotate(err, "GetJobByUUID")
	}

	return job.Job{
		UUID:          uuid_,
		Created:       created.UTC(),
		Settings:      settings,
		StartTime:     startTime,
		EndTime:       endTime,
		Archived:      true,
		ArchiveReason: &archiveReason,
	}, true, nil
}

func (db *DB) GetNextJob() (job.Job, bool, error) {
	// TODO pass context from caller
	tx, err := db.db.BeginTx(context.Background(), nil)
	if err != nil {
		return job.Job{}, false, errors.Trace(err)
	}
	defer tx.Rollback()

	// First look for running jobs, in case we crashed
	row := db.db.QueryRow(`
	SELECT uuid, created, settings, start_time, end_time
	FROM jobs WHERE running=true
	ORDER BY created ASC
	LIMIT 1
	`)

	j, found, err := scanJobRow(row)
	if err != nil {
		return job.Job{}, false, errors.Trace(err)
	}
	if found {
		return j, true, nil
	}

	// Next look for the most recently created job and update it to running=true
	row = db.db.QueryRow(`
		UPDATE jobs
			SET running=true, start_time=$1
		FROM (
			SELECT uuid, created, settings, start_time, end_time
			FROM jobs
			WHERE running=false
			ORDER BY created ASC
			LIMIT 1
		) a
		RETURNING a.*
	`, time.Now().UTC())

	return scanJobRow(row)
}

func (db *DB) ArchiveJob(ar job.ArchiveReason, uuid_ uuid.UUID, endTime time.Time) error {
	result, err := db.db.Exec(`
	WITH moved_row AS (
    DELETE FROM jobs a
		WHERE a.uuid=$1
		RETURNING a.id, a.uuid, a.created, a.settings, a.start_time, $2::timestamptz, $3::archive_reason
	)
	INSERT INTO jobs_archive (id, uuid, created, settings, start_time, end_time, archive_reason)
		SELECT * FROM moved_row
	`, uuid_, endTime, ar)
	if err != nil {
		return errors.Annotate(err, "ArchiveJob Query")
	}

	ra, err := result.RowsAffected()
	if err != nil {
		return errors.Annotate(err, "ArchiveJob RowsAffected")
	}
	if ra != 1 {
		return errors.New(fmt.Sprintf("ArchieJob affected a wrong number of rows (%d)", ra))
	}

	return nil
}

func (db *DB) GetAllJobs() ([]job.Job, error) {
	rows, err := db.db.Query(`
	SELECT uuid, created, settings, start_time, end_time FROM jobs
	ORDER BY created ASC
	`)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer rows.Close()

	jobs := []job.Job{}
	for rows.Next() {
		var (
			uuid_     uuid.UUID
			created   time.Time
			settings  job.Settings
			startTime *time.Time
			endTime   *time.Time
		)
		rows.Scan(&uuid_, &created, &settings, &startTime, &endTime)
		jobs = append(jobs, job.Job{
			UUID:      uuid_,
			Created:   created.UTC(),
			Settings:  settings,
			StartTime: startTime,
			EndTime:   endTime,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Trace(err)
	}

	return jobs, nil
}

func scanJobRow(row *sql.Row) (job.Job, bool, error) {
	var (
		uuid_     uuid.UUID
		created   time.Time
		settings  job.Settings
		startTime *time.Time
		endTime   *time.Time
	)

	if err := row.Err(); err != nil {
		return job.Job{}, false, errors.Annotate(err, "GetNextJob")
	}

	if err := row.Scan(&uuid_, &created, &settings, &startTime, &endTime); err == sql.ErrNoRows {
		return job.Job{}, false, nil
	} else if err != nil {
		return job.Job{}, false, errors.Annotate(err, "scanJobRow")
	}

	return job.Job{
		UUID:      uuid_,
		Created:   created.UTC(),
		Settings:  settings,
		StartTime: startTime,
		EndTime:   endTime,
	}, true, nil
}

func GetTestConnection() (*DB, error) {
	db, err := Connect("ai-art-db", 5432, "puma", "admin", "puma")
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = db.Reset()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return db, nil
}
