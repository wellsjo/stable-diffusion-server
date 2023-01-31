package api

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/wellsjo/ai-art/server/db"
	"github.com/wellsjo/ai-art/server/job"
	"github.com/wellsjo/ai-art/server/job_manager"
	"github.com/wellsjo/ai-art/server/ws"
)

type API struct {
	opts       Opts
	db         *db.DB
	jobManager *job_manager.JobManager
	wsManager  *ws.WSManager
	router     *gin.Engine
	timeout    time.Duration
}

type Opts struct {
	UploadPath string
	MockJobs   bool
	UseS3      bool
	Port       int
}

const DefaultPort = 8080
const DefaultMaxMemory = 32 << 20

func New(
	opts Opts,
	jobManager *job_manager.JobManager,
	wsManager *ws.WSManager,
	db *db.DB,
) *API {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.MaxMultipartMemory = DefaultMaxMemory // 32 MB

	if opts.Port == 0 {
		opts.Port = DefaultPort
	}
	if opts.UploadPath == "" {
		opts.UploadPath = "/home/wells/src/ai-art/stable-diffusion-docker/input"
	}

	a := &API{
		router:     r,
		jobManager: jobManager,
		wsManager:  wsManager,
		db:         db,
		timeout:    2 * time.Second,
		opts:       opts,
	}
	a.setRoutes()

	return a
}

func (a *API) setRoutes() {

	a.router.GET("/ws", func(c *gin.Context) {
		if err := a.wsManager.AddConnection(c); err != nil {
			log.Println(errors.ErrorStack(err))
			errorResponse(err, 500, c)
		}
	})

	a.router.GET("/", func(c *gin.Context) {
		c.HTML(
			// Set the HTTP status to 200 (OK)
			http.StatusOK,
			// Use the index.html template
			"index.html",
			// Pass the data that the page uses (in this case, 'title')
			gin.H{
				"title": "AI ART - HOME",
			},
		)
	})

	a.router.POST("/job", func(c *gin.Context) {
		err := c.Request.ParseMultipartForm(DefaultMaxMemory)
		if err != nil {
			errorResponse(err, 500, c)
			return
		}

		var (
			prompt, widthStr, heightStr, numIterStr string
		)

		log.Println("POST FORM", c.Request.PostForm)
		for key, value := range c.Request.PostForm {
			if key == "prompt" && len(value) > 0 {
				prompt = value[0]
			}
			if key == "width" {
				widthStr = value[0]
			}
			if key == "height" {
				heightStr = value[0]
			}
			if key == "num-iter" {
				numIterStr = value[0]
			}
		}

		var numIter int64 = 0
		if numIterStr != "" {
			var err error
			numIter, err = strconv.ParseInt(numIterStr, 10, 0)
			if err != nil {
				errorResponse(err, 400, c)
				return
			}
		}

		var width int64 = 0
		if widthStr != "" {
			var err error
			width, err = strconv.ParseInt(widthStr, 10, 0)
			if err != nil {
				errorResponse(err, 400, c)
				return
			}
		}

		var height int64 = 0
		if heightStr != "" {
			var err error
			height, err = strconv.ParseInt(heightStr, 10, 0)
			if err != nil {
				errorResponse(err, 400, c)
				return
			}
		}

		j, err := job.New(job.Settings{
			Prompt:        prompt,
			Width:         int(width),
			Height:        int(height),
			NumIterations: int(numIter),
		})
		// TODO capture other types of errors
		if err != nil {
			errorResponse(err, 500, c)
			return
		}

		jobMode := job.TextToImageMode
		imageFile, err := c.FormFile("image")
		if err == nil {
			uploadPath := filepath.Join(a.opts.UploadPath, j.UUID.String())
			if err = c.SaveUploadedFile(imageFile, uploadPath); err != nil {
				errorResponse(err, 500, c)
				return
			}
			jobMode = job.ImageToImageMode
		}

		j.Settings.Mode = jobMode

		// Adds job to queue
		if err := a.db.AddJob(j); err != nil {
			errorResponse(err, 500, c)
			return
		}

		url := fmt.Sprintf("/job/%v", j.UUID)

		c.Redirect(http.StatusFound, url)
	})

	a.router.GET("/job/:uuid", func(c *gin.Context) {
		uuid_ := c.Param("uuid")
		parsedUUID, err := uuid.Parse(uuid_)

		j, found, _, err := a.jobManager.GetJobStatus(parsedUUID, a.timeout)
		if !found {
			errorResponse(ErrJobNotFound, 404, c)
			return
		}
		if err != nil {
			errorResponse(err, 500, c)
			return
		}

		// finalJobDur := time.Duration(0)
		runningDurSecs := time.Duration(0)
		if j.Running {
			runningDurSecs = time.Now().Sub(*j.StartTime).Truncate(time.Second)
		} else if j.EndTime != nil {
			// finalJobDur = (*j.EndTime).Sub(*j.StartTime)
		}

		ar := ""
		if j.ArchiveReason != nil {
			ar = string(*j.ArchiveReason)
		}

		log.Println("Get", j)
		log.Println("ArchiveReason", ar)

		imgURL := ""
		if a.opts.MockJobs {
			imgURL = fmt.Sprintf("/image/w/mock.png")
		} else if a.opts.UseS3 {
			// TODO put in config
			imgURL = fmt.Sprintf("https://ai-art-1.s3.amazonaws.com/%v.png", j.UUID)
		} else {
			imgURL = fmt.Sprintf("/image/sd/%v.png", j.UUID)
		}

		c.HTML(
			http.StatusOK,
			"job.html",
			gin.H{
				"title":   "AI ART - JOB",
				"imgURL":  imgURL,
				"jobDone": j.Done(),
				// "finalJobDur": finalJobDur,
				"runningDurSecs": runningDurSecs,
				"job":            j,
			},
		)
	})

	a.router.Static("/image/w", "./images")
	a.router.Static("/image/sd", "./stable-diffusion-docker/output")
	a.router.Static("/js", "./js")
}

var ErrJobNotFound = errors.New("job not found")

func errorResponse(err error, code int, c *gin.Context) {
	log.Println("Error", errors.ErrorStack(err))
	c.JSON(code, map[string]interface{}{
		"error": err.Error(),
	})
}

func (a *API) Run() {
	log.Println("RUNNING", a.opts.Port)
	a.router.Run(fmt.Sprintf(":%d", a.opts.Port))
}
