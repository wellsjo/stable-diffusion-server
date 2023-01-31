package job

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/juju/errors"
)

const DEFAULT_WIDTH = 512
const DEFAULT_HEIGHT = 512
const DEFAULT_NUM_ITERATIONS = 1

type Mode int

const (
	TextToImageMode Mode = iota
	ImageToImageMode
)

type Settings struct {
	Prompt        string `json:"prompt"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	NumIterations int    `json:"numIterations"`
	Mode          Mode   `json:"mode"`
}

func (s Settings) String() string {
	b, _ := json.MarshalIndent(s, "", "	")
	return string(b)
}

func (s Settings) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *Settings) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &s)
}

type Job struct {
	Settings      Settings
	UUID          uuid.UUID
	Running       bool
	Created       time.Time
	StartTime     *time.Time
	EndTime       *time.Time
	Archived      bool
	ArchiveReason *ArchiveReason
}

func (j Job) Pending() bool {
	return !j.Archived && !j.Running
}

func (j Job) Done() bool {
	return j.Archived && *j.ArchiveReason == ArchiveReasonDone
}

func (j Job) Status() string {
	if j.Running {
		return "running"
	}
	if j.Pending() {
		return "pending"
	}
	if j.Archived {
		return string(*j.ArchiveReason)
	}
	panic("invalid state")
}

func New(settings Settings) (Job, error) {
	if settings.Prompt == "" {
		return Job{}, errors.Trace(errors.New("missing prompt"))
	}

	if settings.NumIterations <= 0 {
		settings.NumIterations = DEFAULT_NUM_ITERATIONS
		log.Println("using default num iterations", DEFAULT_NUM_ITERATIONS)
	}
	if settings.Width <= 0 {
		settings.Width = DEFAULT_WIDTH
		log.Println("using default width setting", DEFAULT_WIDTH)
	}
	if settings.Height <= 0 {
		settings.Height = DEFAULT_HEIGHT
		log.Println("using default height setting", DEFAULT_HEIGHT)
	}

	if err := dimensionValid(settings.Width); err != nil {
		return Job{}, errors.Trace(err)
	}
	if err := dimensionValid(settings.Height); err != nil {
		return Job{}, errors.Trace(err)
	}

	return Job{
		Settings: settings,
		UUID:     uuid.New(),
		Created:  time.Now().Truncate(time.Microsecond).UTC(),
	}, nil
}

func dimensionValid(dim int) error {
	if dim%8 != 0 {
		return errors.Errorf("invalid dimension %v", dim)
	}
	return nil
}

func (j Job) String() string {
	return fmt.Sprintf("%v (%v) '%v' created %v", j.UUID, j.Status(), j.Settings, j.Created)
}

type ArchiveReason string

var (
	ArchiveReasonDone      ArchiveReason = "done"
	ArchiveReasonCancelled ArchiveReason = "cancelled"
	ArchiveReasonError     ArchiveReason = "error"
)

func (a ArchiveReason) String() string {
	return string(a)
}
