package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
	"github.com/wellsjo/ai-art/server/api"
	"github.com/wellsjo/ai-art/server/db"
	"github.com/wellsjo/ai-art/server/job_manager"
	"github.com/wellsjo/ai-art/server/s3_manager"
	"github.com/wellsjo/ai-art/server/ws"
)

func main() {
	var (
		helpOption                bool
		apiPortOption             int
		mockJobsOption            bool
		useS3Option               bool
		s3BucketOption            string
		s3RegionOption            string
		awsAccessKeyOption        string
		awsSecretAccessKeyOption  string
		useCPUOption              bool
		maxNumIterationsOption    int
		stableDiffusionPathOption string
	)

	defaultSDPath := ""
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logFatalError(err)
	}
	defaultSDPath = filepath.Join(homeDir, "src", "ai-art", "stable-diffusion-docker")

	flag.BoolVar(&helpOption, "help", false, "print arg descriptions")
	flag.IntVar(&apiPortOption, "api-port", 8080, "REST api port")
	flag.BoolVar(&useS3Option, "use-s3", false, "if true, upload images to s3. otherwise, use local disk")
	flag.StringVar(&s3BucketOption, "s3-bucket", "", "s3 bucket to use")
	flag.StringVar(&s3RegionOption, "s3-region", "", "s3 region to use")
	flag.StringVar(&awsAccessKeyOption, "aws-access-key", "", "aws access key to use for s3")
	flag.StringVar(&awsSecretAccessKeyOption, "aws-secret-access-key", "", "aws secret access key to use for s3")
	flag.BoolVar(&useCPUOption, "use-cpu", false, "use cpu if gpu is not supported")
	flag.BoolVar(&mockJobsOption, "mock-jobs", false, "mock image creation jobs for testing")
	flag.IntVar(&maxNumIterationsOption, "max-num-iterations", 50, "maximum number of iterations for stable diffusion to use per job")
	flag.StringVar(&stableDiffusionPathOption, "stable-diffusion-path", defaultSDPath, "path to stable diffusion docker entrypoint")
	flag.Parse()

	if helpOption {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if mockJobsOption && useS3Option {
		panic("cannot upload to s3 if using --mock-jobs option")
	}

	if useS3Option {
		if s3BucketOption == "" {
			panic("missing --s3-bucket")
		}
		if s3RegionOption == "" {
			panic("missing --s3-region")
		}
		if awsAccessKeyOption == "" {
			panic("missing --aws-access-key")
		}
		if awsSecretAccessKeyOption == "" {
			panic("missing --aws-secret-access-key")
		}
	}

	renderingHardware := ""
	if mockJobsOption {
		renderingHardware = "mocked"
	} else if useCPUOption {
		renderingHardware = "cpu"
	} else {
		renderingHardware = "gpu"
	}

	saveFilesTo := "locally"
	if useS3Option {
		saveFilesTo = fmt.Sprintf("s3://%s (%s)", s3BucketOption, s3RegionOption)
	}

	log.Println("Save Files:", saveFilesTo)
	log.Println("Rendering Hardware:", renderingHardware)
	log.Println("Stable Diffusion Path:", stableDiffusionPathOption)
	log.Println("Stable Diffusion Num Iterations:", maxNumIterationsOption)

	// Debug logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lshortfile)

	// TODO make these configurable
	db, err := db.Connect("ai-art-db", 5432, "puma", "admin", "puma")
	if err != nil {
		logFatalError(err)
	}

	s3Manager, err := s3_manager.New(s3_manager.Opts{
		Bucket:          s3BucketOption,
		Region:          s3RegionOption,
		AccessKey:       awsAccessKeyOption,
		SecretAccessKey: awsSecretAccessKeyOption,
	})
	if err != nil {
		logFatalError(err)
	}

	wsManager := ws.NewWSManager()

	jobManager := job_manager.New(
		job_manager.Opts{
			MockJobs:            mockJobsOption,
			UseCPU:              useCPUOption,
			UseS3:               useS3Option,
			StableDiffusionPath: stableDiffusionPathOption,
			MaxNumIterations:    maxNumIterationsOption,
		},
		db,
		s3Manager,
		wsManager,
	)
	jobManager.Run()

	server := api.New(
		api.Opts{
			MockJobs: mockJobsOption,
			Port:     apiPortOption,
			UseS3:    useS3Option,
		},
		jobManager,
		wsManager,
		db,
	)

	server.Run()
}

func logFatalError(err error) {
	log.Fatal(errors.ErrorStack(err))
}
