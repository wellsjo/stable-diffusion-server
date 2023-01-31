# Stable Diffusion Server

This is a web app for running and viewing stable diffusion jobs. It works with both text-to-image and image-to-image models. You may specify an S3 bucket to upload images to, or save them locally. Each image generation job can take parameters for prompt, input image, and the number of iterations for stable diffusion to use.

## Example
![example](./images/example.png)

## Run Locally
```
make setup
make start
make
./bin/stable-diffusion-server
```

## Server Run Options
```
stable-diffusion-server [options]

Options:
  --api-port                       REST api port (default 8080)
  --stable-diffusion-path          path to stable diffusion docker entrypoint (default "/home/wells/src/ai-art/stable-diffusion-docker")
  --mock-jobs                      mock image creation jobs for testing
  --use-cpu                        use cpu instead of gpu (fixes compatibility issues)
  --aws-access-key                 aws access key to use for s3
  --aws-secret-access-key          aws secret access key to use for s3
  --max-num-iterations             maximum number of iterations for stable diffusion to use per job (default 50)
  --s3-bucket                      s3 bucket to use
  --s3-region                      s3 region to use
  --use-s3                         if true, upload images to s3. otherwise, use local disk
 ```
