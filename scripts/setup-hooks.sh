#!/bin/bash

set -e

echo "Setting up git hooks..."

# Install pre-commit if not already installed
if ! command -v pre-commit &> /dev/null; then
    echo "Installing pre-commit..."
    pip install pre-commit || pip3 install pre-commit
fi

# Install pre-commit hooks
pre-commit install
pre-commit install --hook-type commit-msg

# Create commit-msg hook for conventional commits
cat > .git/hooks/commit-msg << 'EOF'
#!/bin/bash

# Conventional Commits pattern
commit_regex='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?: .{1,50}'

if ! grep -qE "$commit_regex" "$1"; then
    echo "Aborting commit. Your commit message is invalid." >&2
    echo "Please use conventional commit format:" >&2
    echo "  <type>(<scope>): <subject>" >&2
    echo "" >&2
    echo "Example: feat(cache): add SDK analysis endpoint" >&2
    echo "" >&2
    echo "Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert" >&2
    exit 1
fi

# Check for hardcoded values
if grep -E "(localhost:[0-9]+|127\.0\.0\.1:[0-9]+)" "$1"; then
    echo "Aborting commit. Hardcoded localhost URLs found in commit message." >&2
    exit 1
fi
EOF

chmod +x .git/hooks/commit-msg

echo "Git hooks installed successfully!"
echo ""
echo "Pre-commit checks:"
echo "  - Go formatting"
echo "  - Go vet"
echo "  - Go tests"
echo "  - No hardcoded values"
echo "  - No silent error handling"
echo "  - Conventional commit messages"
echo ""
echo "Run 'make pre-commit' to test hooks manually"