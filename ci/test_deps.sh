#!/bin/bash
set -xe

# Store the path to the conex repository to use in the replace directive
CONEX_ROOT=$(pwd)

# Create a temporary directory to clone the boxes
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT
cd "$WORK_DIR"

# Fetch all repositories from the github.com/conex organization
repos=$(curl -s "https://api.github.com/orgs/conex/repos?per_page=100" | grep '"full_name": "conex/' | awk -F'"' '{print $4}' | cut -d/ -f2)

for repo in $repos; do
  git clone "https://github.com/conex/$repo.git"
  cd "$repo"
  
  if [ -f go.mod ]; then
    # Patch to use the conex version from the current branch
    go mod edit -replace github.com/omeid/conex="$CONEX_ROOT"
    
    # Ensure dependencies are tidy after patch
    go mod tidy
    
    # Run tests for the box
    go test -v ./...
  else
    echo "No go.mod found in $repo, skipping..."
  fi
  
  cd ..
done
