#!/bin/sh

set -eu

CWD=$(basename "$PWD")

build() {
  docker build . --tag "$CWD"
}

clean() {
  docker system prune -f
}

runWithGPUs() {
  echo "Running docker with gpus"
  docker run --rm --gpus=all \
      -v huggingface:/home/huggingface/.cache/huggingface \
      -v "$PWD"/input:/home/huggingface/input \
      -v "$PWD"/output:/home/huggingface/output \
      "$CWD" "$@"
}

runWithoutGPUs() {
  echo "Running docker without gpus"
  docker run --rm \
      -v huggingface:/home/huggingface/.cache/huggingface \
      -v "$PWD"/input:/home/huggingface/input \
      -v "$PWD"/output:/home/huggingface/output \
      "$CWD" "$@"
}

mkdir -p input output
case ${1:-build} in
    build) build ;;
    clean) clean ;;
    dev) dev "$@" ;;
    run) shift; run "$@" ;;
    runWithoutGPUs) shift; runWithoutGPUs "$@" ;;
    runWithGPUs) shift; runWithGPUs "$@" ;;
    test) tests ;;
    *) echo "$0: No command named '$1'" ;;
esac
