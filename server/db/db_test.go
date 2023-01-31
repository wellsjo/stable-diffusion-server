package db

import (
	"log"
	"testing"
	"time"

	"github.com/juju/errors"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/wellsjo/ai-art/server/job"
)

func TestDB(t *testing.T) {
	db, err := GetTestConnection()
	if err != nil {
		FatalError(err)
	}

	jobs, err := db.GetAllJobs()
	if err != nil {
		FatalError(err)
	}

	if len(jobs) != 0 {
		log.Fatal("jobs should be 0")
	}

	j1 := NewTestJob("hello")
	assert.Nil(t, err)
	err = db.AddJob(j1)
	if err != nil {
		FatalError(err)
	}
	log.Println("Added Job", j1)

	j1Get, found, _, err := db.GetJobByUUID(j1.UUID)
	assert.Nil(t, err)
	assert.Equal(t, true, found)
	assert.Equal(t, j1, j1Get)

	j2 := NewTestJob("hello2")
	err = db.AddJob(j2)
	if err != nil {
		FatalError(err)
	}
	log.Println("Added Job", j2)

	jobs, err = db.GetAllJobs()
	if err != nil {
		FatalError(err)
	}
	log.Println("Get All Jobs", jobs)

	assert.Equal(t, len(jobs), 2)
	assert.Equal(t, jobs[0], j1)
	assert.Equal(t, jobs[1], j2)

	nextJob, found, err := db.GetNextJob()
	if err != nil {
		FatalError(err)
	}
	log.Println("Next Job", nextJob)

	assert.Equal(t, true, found)
	assert.Equal(t, j1, nextJob)

	log.Println("Archiving", nextJob.UUID)
	err = db.ArchiveJob(job.ArchiveReasonDone, nextJob.UUID, time.Now())
	if err != nil {
		FatalError(err)
	}

	log.Println("GET", nextJob.UUID)
	j, found, _, err := db.GetJobByUUID(nextJob.UUID)
	log.Println("Job", j)

	assert.True(t, j.Archived)
	assert.True(t, found)
	assert.Nil(t, err)
}

func TestGetNextJob(t *testing.T) {
	db, err := GetTestConnection()
	if err != nil {
		FatalError(err)
	}

	j1 := NewTestJob("hello")
	err = db.AddJob(j1)
	if err != nil {
		FatalError(err)
	}

	j2 := NewTestJob("hello2")
	err = db.AddJob(j2)
	if err != nil {
		FatalError(err)
	}

	j, found, err := db.GetNextJob()
	if err != nil {
		FatalError(err)
	}

	assert.Equal(t, true, found)
	assert.Equal(t, j1, j)
	assert.Nil(t, err)

	err = db.ArchiveJob(job.ArchiveReasonDone, j.UUID, time.Now())
	assert.Nil(t, err)

	j, found, err = db.GetNextJob()
	if err != nil {
		FatalError(err)
	}

	// Backfill start time
	j2.StartTime = j.StartTime

	assert.True(t, found)
	assert.Equal(t, j2, j)
	assert.Nil(t, err)

	err = db.ArchiveJob(job.ArchiveReasonDone, j.UUID, time.Now())
	assert.Nil(t, err)

	_, found, err = db.GetNextJob()
	assert.False(t, found)
	assert.Nil(t, err)
}

func TestArchive(t *testing.T) {
	db, err := GetTestConnection()
	if err != nil {
		FatalError(err)
	}

	j := NewTestJob("hello")
	err = db.AddJob(j)
	if err != nil {
		FatalError(err)
	}

	err = db.ArchiveJob(job.ArchiveReasonDone, j.UUID, time.Now())
	if err != nil {
		FatalError(err)
	}

	jobs, err := db.GetAllJobs()
	if err != nil {
		FatalError(err)
	}

	assert.Equal(t, 0, len(jobs))
}

func TestPending(t *testing.T) {
	db, err := GetTestConnection()
	if err != nil {
		FatalError(err)
	}

	j := NewTestJob("hello")
	err = db.AddJob(j)
	if err != nil {
		FatalError(err)
	}

	j2 := NewTestJob("hello2")
	err = db.AddJob(j2)
	if err != nil {
		FatalError(err)
	}

	j3 := NewTestJob("hello3")
	err = db.AddJob(j3)
	if err != nil {
		FatalError(err)
	}

	pos, err := db.getJobQueuePosition(j)
	assert.Nil(t, err)
	assert.Equal(t, 0, pos)

	pos, err = db.getJobQueuePosition(j2)
	assert.Nil(t, err)
	assert.Equal(t, 1, pos)

	pos, err = db.getJobQueuePosition(j3)
	assert.Nil(t, err)
	assert.Equal(t, 2, pos)
}

func NewTestJob(prompt string) job.Job {
	j, _ := job.New(job.Settings{
		Prompt: prompt,
	})
	return j
}

func FatalError(err error) {
	log.Fatal(errors.ErrorStack(err))
}
