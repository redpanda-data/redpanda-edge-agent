#!/bin/bash

HELPTEXT='redpanda-edge-agent build script
  Usage: ./build [flags]

  Flags:
  --archive  -a            Create a compressed archive file (using tar -czf)
  --build    -b PLATFORM   Build for a specific platform (where PLATFORM is linux/amd64, for example)
  --build-all              Build for the following platforms: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
  --help     -h            Show this message
  --include-platform       Include platform in resulting filename (always enabled with --build-all)
  --build-version VERSION  Use the given version when naming the archive file
  --verbose  -v            Print task details
'

PLATFORMS=("`go env GOOS`/`go env GOARCH`")
DEFAULT_PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64")
FILENAME="redpanda-edge-agent"
INCLUDE_PLATFORM=false
ARCHIVE=false
VERBOSE=false

while [ $# -gt 0 ]; do
  case $1 in
  -h | --help) echo "$HELPTEXT"; exit;;
  -a | --archive) ARCHIVE=true;shift;;
  -b | --build) PLATFORMS=("$2");shift 2;;
  --build-all) PLATFORMS=(${DEFAULT_PLATFORMS[*]});INCLUDE_PLATFORM=true;shift;;
  --build-version) VERSION=$2;shift 2;;
  --include-platform) INCLUDE_PLATFORM=true;shift;;
  -v | --verbose) VERBOSE=true;shift;;
  *) break
  esac
done

# determine if script is ran from ./docker or ./
AGENT_PATH="./agent/"
CWD=${PWD##*/}
CWD=${CWD:-/}
if [ $CWD == docker ]; then
  AGENT_PATH=".$AGENT_PATH"
fi

for i in "${PLATFORMS[@]}"
do
  if [ $VERBOSE == true ]; then
    echo "Building release for $i..."
  fi
  platform=(${i//\// })
  if [ $INCLUDE_PLATFORM == true ]; then
    FILENAME=$FILENAME-${platform[0]}-${platform[1]}
  fi
  if [ $VERSION ]; then
    FILENAME=$FILENAME-$VERSION
  fi
  
  GOOS=${platform[0]} GOARCH=${platform[1]} go build -a -o $FILENAME $AGENT_PATH
  if [ ! -f "${FILENAME}" ]; then
      echo "Error building file: ${FILENAME}"
      exit 1
  fi

  if [ $ARCHIVE == true ]; then
    archive="$FILENAME.tar.gz"
    tar -czf ${archive} $FILENAME
    if [ $VERBOSE == true ]; then
    echo "$archive created"
    fi
    rm $FILENAME
  else
    if [ $VERBOSE == true ]; then
      echo "$FILENAME created"
    fi
  fi
done
