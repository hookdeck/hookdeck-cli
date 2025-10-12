#!/bin/sh
set -e

# Generate shell completions for Hookdeck CLI
# This script is run during the GoReleaser build process to pre-generate
# completion files that will be included in the release archives.

rm -rf completions
mkdir completions

# Use 'go run .' to compile and run the CLI to generate completions
# This works on any platform that can build Go code
# The completion command writes files to the current directory, so we cd into completions/
echo "Generating bash completion..."
(cd completions && go run .. completion --shell bash)

echo "Generating zsh completion..."
(cd completions && go run .. completion --shell zsh)

# Rename the generated files to match GoReleaser expectations
mv completions/hookdeck-completion.bash completions/hookdeck.bash
mv completions/hookdeck-completion.zsh completions/_hookdeck

# Fish completion is not currently supported by the CLI
# If it gets added in the future, uncomment this:
# echo "Generating fish completion..."
# go run . completion --shell fish > completions/hookdeck.fish

echo "✅ Completions generated successfully in completions/"
ls -lh completions/