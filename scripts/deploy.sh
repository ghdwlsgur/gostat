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

  # Everything that can be checked is checked before the tag exists, because
  # a pushed tag is what starts the image workflow and is not something to
  # take back afterwards.
  goreleaser check

  if [ -z "$GITHUB_TOKEN" ] && command -v gh >/dev/null 2>&1; then
    GITHUB_TOKEN=$(gh auth token 2>/dev/null) || true
    export GITHUB_TOKEN
  fi

  if [ -z "$GITHUB_TOKEN" ]; then
    echo "GITHUB_TOKEN is unset and gh has no token to lend; goreleaser could"
    echo "not publish, and the tag would already be pushed by then."
    exit 1
  fi

  if git rev-parse "$tag" >/dev/null 2>&1; then
    echo "tag $tag already exists"
    exit 1
  fi

  # goreleaser runs the suite too, but from its before-hooks, which is after
  # the tag has been created and pushed. A failure there leaves a tag behind
  # with no release to go with it, and the image workflow already started.
  # Run it here, while backing out still costs nothing.
  test

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
