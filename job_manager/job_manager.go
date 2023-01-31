package job_manager

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/wellsjo/ai-art/db"
	"github.com/wellsjo/ai-art/job"
	"github.com/wellsjo/ai-art/s3_manager"
	"github.com/wellsjo/ai-art/ws"
)

const NEXT_JOB_THROTTLE = 1 * time.Second
const MAX_NUM_ITERATIONS = 50

type statusRequest struct {
	uuid uuid.UUID
}

type statusResponse struct {
	found    bool
	position int
	job      job.Job
}

type JobManager struct {
	opts Opts

	statusRequests  chan statusRequest
	statusResponses chan statusResponse

	addJobs chan job.Job
	jobDone chan job.Job
	queue   chan job.Job
	done    chan struct{}

	ws *ws.WSManager
	s3 *s3_manager.S3Manager
	db *db.DB
}

type Opts struct {
	MockJobs            bool
	UseS3               bool
	UseCPU              bool
	StableDiffusionPath string
	ImageUploadPath     string
	MaxNumIterations    int
}

func New(
	opts Opts,
	db *db.DB,
	s3m *s3_manager.S3Manager,
	wsm *ws.WSManager,
) *JobManager {
	if opts.MaxNumIterations == 0 {
		opts.MaxNumIterations = MAX_NUM_ITERATIONS
	}

	return &JobManager{
		opts: opts,

		statusRequests:  make(chan statusRequest),
		statusResponses: make(chan statusResponse),

		addJobs: make(chan job.Job),
		jobDone: make(chan job.Job),
		queue:   make(chan job.Job, 100),
		done:    make(chan struct{}),

		ws: wsm,
		s3: s3m,
		db: db,
	}
}

func (jm JobManager) Run() {
	go jm.RunJobsLoop()

	go func() {
		for {
			select {
			// case j := <-jm.addJobs:
			// 	// TODO return error to caller
			// 	err := jm.db.AddJob(j)
			// 	if err != nil {
			// 		log.Println(errors.ErrorStack(err))
			// 	}

			case j := <-jm.jobDone:
				log.Println("Job Done", j)
				err := jm.db.ArchiveJob(job.ArchiveReasonDone, j.UUID, *j.EndTime)
				if err != nil {
					log.Println(errors.ErrorStack(err))
					return
				}

				fileName := jm.getJobFileName(j.UUID)
				imagePath := filepath.Join(jm.opts.StableDiffusionPath, "output", fileName)

				if jm.opts.UseS3 {
					if err = jm.s3.UploadFile(imagePath, fileName); err != nil {
						log.Println(errors.ErrorStack(err))
						// return
					}

					if err = os.Remove(imagePath); err != nil {
						fmt.Println(err)
						// return
					}
				}

				if err = jm.ws.Broadcast(j.UUID, ws.Message{"job": "done"}); err != nil {
					log.Println(errors.ErrorStack(err))
					// return
				}

			case req := <-jm.statusRequests:
				log.Println("Status Request", req.uuid.String())
				job, found, pos, err := jm.db.GetJobByUUID(req.uuid)
				if err != nil {
					log.Println(errors.ErrorStack(err))
					return
				}

				sr := statusResponse{
					found:    found,
					position: pos,
					job:      job,
				}

				log.Println("Status Response", sr.job)
				jm.statusResponses <- sr

			case <-jm.done:
				log.Println("Done signal received. Closing JobManager")
				close(jm.queue)
				return
			}
		}
	}()
}

func (jm JobManager) RunJobsLoop() {
	for {
		j, found, err := jm.db.GetNextJob()
		if err != nil {
			log.Fatal(err)
		}

		if !found {
			time.Sleep(NEXT_JOB_THROTTLE)
			continue
		}

		jm.ws.Broadcast(j.UUID, ws.Message{"job": "running"})

		startTime := time.Now()
		j.StartTime = &startTime

		if !jm.opts.MockJobs {
			log.Println("Running job", j)
			err := jm.RunStableDiffusionJob(j)
			if err != nil {
				log.Println("Job Error", errors.ErrorStack(err))
				// TODO
			}
		} else {
			log.Println("Running mock job", j)
			jm.RunStableDiffusionJobMock()
		}

		endTime := time.Now()
		j.EndTime = &endTime

		jm.jobDone <- j
	}

	log.Println("Done running jobs")
}

func (jm JobManager) Close() {
	log.Println("Closing done chan")
	close(jm.done)
}

func (jm *JobManager) AddJob(j job.Job, timeout time.Duration) error {
	if j.Settings.NumIterations > jm.opts.MaxNumIterations {
		return errors.Errorf("NumIterations setting is too high (max %v)", jm.opts.MaxNumIterations)
	}

	select {
	case jm.addJobs <- j:
	case <-time.After(timeout):
		return errors.New("Failed to add job (timeout)")
	}

	return nil
}

func (jm *JobManager) GetJobStatus(uuid uuid.UUID, timeout time.Duration) (job.Job, bool, int, error) {
	sr := statusRequest{
		uuid: uuid,
	}

	select {
	case jm.statusRequests <- sr:
		sr := <-jm.statusResponses
		return sr.job, sr.found, sr.position, nil

	case <-time.After(timeout):
		return job.Job{}, false, 0, errors.New("Failed to get status (timeout)")
	}
}

func (jm *JobManager) RunStableDiffusionJob(j job.Job) error {
	start := time.Now()
	fileName := jm.getJobFileName(j.UUID)

	var (
		cmd       *exec.Cmd
		cmdName   = "./build.sh"
		cmdFnName = "runWithGPUs"
		args      []string
	)

	if jm.opts.UseCPU {
		cmdFnName = "runWithoutGPUs"
	}

	args = []string{
		cmdFnName,
	}

	// Image-To-Image Mode
	if j.Settings.Mode == job.ImageToImageMode {
		args = append(args,
			"--image",
			filepath.Join(jm.opts.ImageUploadPath, fmt.Sprintf("%v", j.UUID)),
			"--strength",
			"0.5",
		)
	}

	args = append(args, []string{
		j.Settings.Prompt,
		"--n_iter", fmt.Sprintf("%d", j.Settings.NumIterations),
		"--output", fileName,
		"--W", fmt.Sprintf("%d", j.Settings.Width),
		"--H", fmt.Sprintf("%d", j.Settings.Height),
	}...)
	log.Println("Running Command", cmdName, args)

	cmd = exec.Command(
		cmdName,
		args...,
	)

	cmd.Dir = jm.opts.StableDiffusionPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Annotate(err, "RunStableDiffusionJob StdoutPipe")
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Annotate(err, "RunStableDiffusionJob StderrPipe")
	}

	log.Println("Starting to build image", j.UUID)
	err = cmd.Start()
	if err != nil {
		return errors.Annotate(err, "RunStableDiffusionJob Start")
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	log.Println("Waiting...")
	err = cmd.Wait()
	if err != nil {
		return errors.Annotate(err, "RunStableDiffusionJob Wait")
	}

	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			break
		}
	}
	for {
		tmp := make([]byte, 1024)
		_, err := stderr.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			break
		}
	}

	log.Println("Job Done", time.Now().Sub(start))
	return nil
}

func (jm *JobManager) getJobFileName(uuid_ uuid.UUID) string {
	return fmt.Sprintf("%s.png", uuid_.String())
}

func (jm *JobManager) RunStableDiffusionJobMock() {
	time.Sleep(3 * time.Second)
}
