#!/bin/bash

# DevOps MCP - IDE Problem Diagnostic Script
# This script helps identify all potential issues that IDEs might report

echo "========================================="
echo "DevOps MCP - Comprehensive Issue Scanner"
echo "========================================="
echo ""

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Counter for total issues
TOTAL_ISSUES=0

# Create temporary directory for results
RESULTS_DIR="/tmp/devops-mcp-diagnosis-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo "Results will be saved to: $RESULTS_DIR"
echo ""

# Function to count and display issues
count_issues() {
    local count=$1
    local description=$2
    TOTAL_ISSUES=$((TOTAL_ISSUES + count))
    if [ $count -gt 0 ]; then
        echo -e "${RED}[FOUND]${NC} $count $description"
    else
        echo -e "${GREEN}[OK]${NC} No $description"
    fi
}

# 1. Compilation Errors
echo "1. Checking for compilation errors..."
COMPILE_ERRORS=$(find . -name "*.go" -not -path "./vendor/*" -exec go build {} 2>&1 \; | grep -E "\.go:[0-9]+:" | wc -l)
find . -name "*.go" -not -path "./vendor/*" -exec go build {} 2>&1 \; | grep -E "\.go:[0-9]+:" > "$RESULTS_DIR/compilation_errors.txt" 2>/dev/null
count_issues $COMPILE_ERRORS "compilation errors"
echo ""

# 2. Go Vet Issues
echo "2. Running go vet..."
GO_VET_ISSUES=0
for module in pkg apps/edge-mcp apps/rest-api apps/worker apps/mockserver; do
    if [ -d "$module" ]; then
        cd "$module" 2>/dev/null && go vet ./... 2>&1 | tee -a "$RESULTS_DIR/go_vet.txt" | grep -v "^#" | wc -l | read module_issues
        GO_VET_ISSUES=$((GO_VET_ISSUES + module_issues))
        cd - > /dev/null 2>&1
    fi
done
count_issues $GO_VET_ISSUES "go vet issues"
echo ""

# 3. Golangci-lint Issues
echo "3. Running golangci-lint..."
if command -v golangci-lint &> /dev/null; then
    LINT_ISSUES=$(make lint 2>&1 | tee "$RESULTS_DIR/golangci_lint.txt" | grep -E "\.go:[0-9]+:" | wc -l)
    count_issues $LINT_ISSUES "golangci-lint issues"
else
    echo -e "${YELLOW}[SKIP]${NC} golangci-lint not installed"
fi
echo ""

# 4. Inefficient Assignments
echo "4. Checking for inefficient assignments..."
if command -v ineffassign &> /dev/null; then
    INEFFASSIGN_ISSUES=$(ineffassign ./... 2>&1 | tee "$RESULTS_DIR/ineffassign.txt" | wc -l)
    count_issues $INEFFASSIGN_ISSUES "inefficient assignments"
else
    echo -e "${YELLOW}[SKIP]${NC} ineffassign not installed (go install github.com/gordonklaus/ineffassign@latest)"
fi
echo ""

# 5. Misspellings
echo "5. Checking for misspellings..."
if command -v misspell &> /dev/null; then
    MISSPELL_ISSUES=$(misspell -error . 2>&1 | tee "$RESULTS_DIR/misspell.txt" | wc -l)
    count_issues $MISSPELL_ISSUES "misspellings"
else
    echo -e "${YELLOW}[SKIP]${NC} misspell not installed (go install github.com/client9/misspell/cmd/misspell@latest)"
fi
echo ""

# 6. TODO/FIXME Comments
echo "6. Counting TODO/FIXME comments..."
TODO_COUNT=$(grep -r "TODO\|FIXME\|XXX\|HACK" --include="*.go" . 2>/dev/null | tee "$RESULTS_DIR/todos.txt" | wc -l)
count_issues $TODO_COUNT "TODO/FIXME comments"
echo ""

# 7. Missing Package Comments
echo "7. Checking for missing package comments..."
MISSING_PKG_COMMENTS=$(find . -name "*.go" -not -path "./vendor/*" -not -name "*_test.go" | xargs grep -L "^// Package" | tee "$RESULTS_DIR/missing_package_comments.txt" | wc -l)
count_issues $MISSING_PKG_COMMENTS "files missing package comments"
echo ""

# 8. Exported Functions Without Comments
echo "8. Checking for exported functions without comments..."
MISSING_FUNC_COMMENTS=$(grep -r "^func [A-Z]" --include="*.go" . 2>/dev/null | grep -v "//" | tee "$RESULTS_DIR/missing_func_comments.txt" | wc -l)
count_issues $MISSING_FUNC_COMMENTS "exported functions without comments"
echo ""

# 9. Long Lines
echo "9. Checking for long lines (>120 chars)..."
LONG_LINES=$(find . -name "*.go" -not -path "./vendor/*" -exec awk 'length > 120 {print FILENAME":"NR":Line too long ("length" chars)"}' {} \; | tee "$RESULTS_DIR/long_lines.txt" | wc -l)
count_issues $LONG_LINES "long lines"
echo ""

# 10. Deprecated Function Usage
echo "10. Checking for deprecated function usage..."
DEPRECATED=$(grep -r "Deprecated:" --include="*.go" . 2>/dev/null | tee "$RESULTS_DIR/deprecated.txt" | wc -l)
count_issues $DEPRECATED "deprecated function usages"
echo ""

# 11. Import Issues
echo "11. Checking for import issues..."
IMPORT_ISSUES=$(goimports -l . 2>/dev/null | tee "$RESULTS_DIR/import_issues.txt" | wc -l)
if [ -z "$IMPORT_ISSUES" ]; then
    IMPORT_ISSUES=0
fi
count_issues $IMPORT_ISSUES "import formatting issues"
echo ""

# 12. Cyclomatic Complexity
echo "12. Checking cyclomatic complexity..."
if command -v gocyclo &> /dev/null; then
    COMPLEX_FUNCTIONS=$(gocyclo -over 10 . 2>/dev/null | tee "$RESULTS_DIR/complexity.txt" | wc -l)
    count_issues $COMPLEX_FUNCTIONS "functions with high complexity"
else
    echo -e "${YELLOW}[SKIP]${NC} gocyclo not installed (go install github.com/fzipp/gocyclo/cmd/gocyclo@latest)"
fi
echo ""

# 13. Test Coverage Issues
echo "13. Checking for functions without tests..."
# This is a rough estimate - functions without corresponding tests
UNTESTED_FUNCTIONS=$(comm -23 <(grep -r "^func " --include="*.go" . 2>/dev/null | grep -v "_test.go" | cut -d: -f2 | sort -u) <(grep -r "^func Test" --include="*_test.go" . 2>/dev/null | cut -d: -f2 | sed 's/Test//' | sort -u) 2>/dev/null | tee "$RESULTS_DIR/untested_functions.txt" | wc -l)
count_issues $UNTESTED_FUNCTIONS "potentially untested functions"
echo ""

# 14. Security Issues
echo "14. Checking for potential security issues..."
SECURITY_ISSUES=$(grep -r "fmt.Sprintf.*\"select\|\"insert\|\"update\|\"delete" --include="*.go" . 2>/dev/null | tee "$RESULTS_DIR/security_sql.txt" | wc -l)
count_issues $SECURITY_ISSUES "potential SQL injection points"
echo ""

# 15. Type Assertion Issues
echo "15. Checking for unchecked type assertions..."
TYPE_ASSERTIONS=$(grep -r "\.\(" --include="*.go" . 2>/dev/null | grep -v "ok" | tee "$RESULTS_DIR/type_assertions.txt" | wc -l)
count_issues $TYPE_ASSERTIONS "potentially unchecked type assertions"
echo ""

# 16. gopls specific issues
echo "16. Checking gopls diagnostics..."
if command -v gopls &> /dev/null; then
    echo "Running gopls check (this may take a while)..."
    for module in pkg apps/edge-mcp; do
        if [ -d "$module" ]; then
            cd "$module" 2>/dev/null
            gopls check . 2>&1 | tee -a "$RESULTS_DIR/gopls_diagnostics.txt"
            cd - > /dev/null 2>&1
        fi
    done
    GOPLS_ISSUES=$(cat "$RESULTS_DIR/gopls_diagnostics.txt" | grep -E "\.go:[0-9]+:" | wc -l)
    count_issues $GOPLS_ISSUES "gopls diagnostics"
else
    echo -e "${YELLOW}[SKIP]${NC} gopls not installed"
fi
echo ""

# Summary
echo "========================================="
echo -e "${YELLOW}SUMMARY${NC}"
echo "========================================="
echo -e "Total issues found: ${RED}$TOTAL_ISSUES${NC}"
echo ""
echo "Detailed results saved in: $RESULTS_DIR"
echo ""
echo "Files generated:"
ls -la "$RESULTS_DIR"
echo ""

# IDE-specific check
echo "========================================="
echo "IDE-SPECIFIC RECOMMENDATIONS"
echo "========================================="
echo ""
echo "If your IDE shows 323 problems but this script shows fewer:"
echo ""
echo "1. Check IDE-specific settings:"
echo "   - VSCode: Check Problems panel filters"
echo "   - GoLand: Check inspection profile settings"
echo "   - Vim: Check ALE or vim-go configuration"
echo ""
echo "2. Clear caches:"
echo "   - gopls: 'gopls cache clean'"
echo "   - go mod: 'go clean -modcache'"
echo "   - IDE: Restart IDE and invalidate caches"
echo ""
echo "3. Update tools:"
echo "   - gopls: 'go install golang.org/x/tools/gopls@latest'"
echo "   - golangci-lint: Check for latest version"
echo ""
echo "4. Check for workspace issues:"
echo "   - Run: 'go work sync'"
echo "   - Verify all modules in go.work exist"
echo ""

# Create summary report
cat > "$RESULTS_DIR/SUMMARY.md" << EOF
# DevOps MCP Diagnostic Report

Generated: $(date)

## Issue Summary
- Total Issues Found: $TOTAL_ISSUES
- Compilation Errors: $COMPILE_ERRORS
- Go Vet Issues: $GO_VET_ISSUES
- Linting Issues: $LINT_ISSUES
- TODO/FIXME Comments: $TODO_COUNT
- Missing Documentation: $((MISSING_PKG_COMMENTS + MISSING_FUNC_COMMENTS))
- Code Quality Issues: $((LONG_LINES + COMPLEX_FUNCTIONS))

## Next Steps
1. Fix compilation errors first (see compilation_errors.txt)
2. Address linting issues (see golangci_lint.txt)
3. Review and fix go vet issues (see go_vet.txt)
4. Add missing documentation
5. Refactor complex functions

## Files Generated
$(ls -1 $RESULTS_DIR)
EOF

echo "Summary report created: $RESULTS_DIR/SUMMARY.md"