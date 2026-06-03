#!/bin/sh
set -e

# Generate shell completions for Hookdeck CLI
# This script is run during the GoReleaser build process to pre-generate
# completion files that will be included in the release archives.

rm -rf completions
mkdir completions

echo "Generating bash completion..."
go run . completion --shell bash > completions/hookdeck.bash

echo "Generating zsh completion..."
go run . completion --shell zsh > completions/_hookdeck

# Fish completion is not currently supported by the CLI
# If it gets added in the future, uncomment this:
# echo "Generating fish completion..."
# go run . completion --shell fish > completions/hookdeck.fish

echo "✅ Completions generated successfully in completions/"
ls -lh completions/
