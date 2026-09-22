#!/usr/bin/env bash 
set -e -o pipefail



trap '[ "$?" -eq 0 ] || echo "Error Line:<$LINENO> Error Function:<${FUNCNAME}>"' EXIT
cd "$(dirname "$0")" && cd ..
CURRENT=$(pwd)

function test
{
    go test ./... --count 1 -race -covermode=atomic -coverprofile="$CURRENT"/coverage.txt
}

function release
{
  tag=$1
  if [ -z "$tag" ]; then
    echo "not found tag name"
    exit 1
  fi

  # Catch a broken config before a tag exists that cannot be taken back.
  goreleaser check

  git tag -a "$tag" -m "Add $tag"
  # Pushing the tag is also what starts the container image workflow.
  git push origin "$tag"

  goreleaser release --clean
}

function release_test
{
  goreleaser release --snapshot --clean --skip=publish
}

CMD=$1
shift
"$CMD" "$*"
