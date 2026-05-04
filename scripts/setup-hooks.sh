#!/bin/bash

# Setup script to configure local Git hooks
# This points Git to the .githooks directory in the repository.

set -e

# Colors for output
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Configuring local Git hooks..."

# Set the hooks path to the version-controlled .githooks directory
git config core.hooksPath .githooks

echo -e "${GREEN}Git hooks successfully configured!${NC}"
echo "Pre-commit checks will now run automatically before every commit."
