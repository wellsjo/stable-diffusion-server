package main

import (
	"log"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
	"github.com/wellsjo/ai-art/api"
	"github.com/wellsjo/ai-art/db"
	"github.com/wellsjo/ai-art/job_manager"
	"github.com/wellsjo/ai-art/s3_manager"
	"github.com/wellsjo/ai-art/ws"
)

func main() {
	var (
		helpOption                bool
		mockJobsOption            bool
		useS3Option               bool
		useCPU                    bool
		maxNumIterationsOption    int
		apiPortOption             int
		stableDiffusionPathOption string
	)

	defaultSDPath := ""
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultSDPath = filepath.Join(homeDir, "src", "ai-art", "stable-diffusion-docker")

	flag.BoolVar(&helpOption, "help", false, "print arg descriptions")
	flag.BoolVar(&mockJobsOption, "mock-jobs", false, "mock image creation jobs for testing")
	flag.IntVar(&maxNumIterationsOption, "max-num-iterations", 50, "maximum number of iterations for stable diffusion to use per job")
	flag.IntVar(&apiPortOption, "api-port", 8080, "REST api port")
	flag.BoolVar(&useS3Option, "use-s3", false, "if true, upload images to s3. otherwise, use local disk")
	flag.BoolVar(&useCPU, "use-cpu", false, "use cpu if gpu is not supported")
	flag.StringVar(&stableDiffusionPathOption, "stable-diffusion-path", defaultSDPath, "path to stable diffusion docker entrypoint")
	flag.Parse()

	if helpOption {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if mockJobsOption && useS3Option {
		panic("cannot upload to s3 if using --mock-jobs option")
	}

	usageStr := ""
	if mockJobsOption {
		usageStr = "mock"
	} else if useCPU {
		usageStr = "cpu"
	} else {
		usageStr = "gpu"
	}

	log.Println("NUM ITERATIONS:", maxNumIterationsOption)
	log.Println("API PORT:", apiPortOption)
	log.Println("USE S3:", useS3Option)
	log.Println("STABLE DIFFUSION:", usageStr)
	log.Println("STABLE DIFFUSION PATH:", stableDiffusionPathOption)

	// Debug logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lshortfile)

	db, err := db.Connect("ai-art-db", 5432, "puma", "admin", "puma")
	if err != nil {
		log.Fatal(err)
	}

	s3Manager, err := s3_manager.New(s3_manager.Opts{
		Bucket: "ai-art-1",
		Region: "us-east-1",
	})
	if err != nil {
		log.Fatal(err)
	}

	wsManager := ws.NewWSManager()

	jobManager := job_manager.New(
		job_manager.Opts{
			MockJobs:            mockJobsOption,
			UseCPU:              useCPU,
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
