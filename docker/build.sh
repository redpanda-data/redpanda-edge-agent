#!/bin/bash

HELPTEXT='redpanda-edge-agent docker build script
  Usage: ./build [flags]

  Flags:
  --username  -u USERNAME  The Docker username for pushing to the registry
  --no-push                The script will not push the build to the registry
  --tag       -t TAG       The build tag (default "latest")
'

FILENAME="redpanda-edge-agent"
PUSH=true
TAG="latest"
USERNAME="redpanda"

while [ $# -gt 0 ]; do
  case $1 in
  -h | --help) echo "$HELPTEXT"; exit;;
  --no-push) PUSH=false;shift;;
  -t | --tag) TAG=$2;shift 2;;
  -u | --username) USERNAME=$2;shift 2;;
  *) break
  esac
done

CWD=${PWD##*/}
CWD=${CWD:-/}
if [ $CWD == docker ]; then
  cd ..
fi

FILENAME="$USERNAME/$FILENAME:$TAG"

docker build -f docker/Dockerfile -t $FILENAME .
if [ $PUSH == true ]; then
  docker push $FILENAME
fi
