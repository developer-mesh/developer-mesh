#!/bin/bash
# Stream 2: Documentation audit pipeline
# This script audits existing documentation for issues

echo "[Stream 2] Starting documentation audit..."

# Create audit directories
mkdir -p .doc-audit/{broken-links,outdated,to-remove,to-update,code-blocks}

# Background task 1: Find all documentation files
echo "[2.1] Cataloging documentation files..."
find . -name "*.md" -type f | grep -v node_modules | grep -v vendor > .doc-audit/all-docs.txt &

# Background task 2: Check for broken internal links
echo "[2.2] Checking for broken internal links..."
(
while IFS= read -r doc; do
    # Extract markdown links
    grep -o '\[.*\]([^)]*\.md[^)]*)' "$doc" 2>/dev/null | while read link; do
        # Extract the path from the link
        target=$(echo "$link" | sed 's/.*(\([^)]*\)).*/\1/' | sed 's/#.*//')
        # Resolve relative paths
        doc_dir=$(dirname "$doc")
        if [[ "$target" == /* ]]; then
            full_path=".$target"
        else
            full_path="$doc_dir/$target"
        fi
        # Normalize the path
        normalized=$(readlink -m "$full_path" 2>/dev/null || echo "$full_path")
        if [[ ! -f "$normalized" ]]; then
            echo "$doc: Broken link to $target (resolved: $normalized)" >> .doc-audit/broken-links/internal.txt
        fi
    done
done < .doc-audit/all-docs.txt
) &

# Background task 3: Find placeholder values
echo "[2.3] Finding placeholder values..."
grep -r "{[^}]*}" --include="*.md" . 2>/dev/null | grep -v node_modules > .doc-audit/outdated/placeholders.txt &

# Background task 4: Find "coming soon" or "TODO" in docs
echo "[2.4] Finding unimplemented documented features..."
grep -ri "coming soon\|todo\|planned\|future\|will be\|upcoming\|TBD\|work in progress" --include="*.md" . 2>/dev/null | \
    grep -v node_modules > .doc-audit/outdated/future-features.txt &

# Background task 5: Extract all code blocks for verification
echo "[2.5] Extracting code blocks for testing..."
(
while IFS= read -r md_file; do
    # Extract bash/shell code blocks
    awk '/```(bash|sh|shell)?$/{flag=1; next} /```/{flag=0} flag' "$md_file" > ".doc-audit/code-blocks/$(basename "$md_file").commands.txt" 2>/dev/null
    
    # Extract curl commands
    grep -o 'curl [^`]*' "$md_file" >> ".doc-audit/code-blocks/$(basename "$md_file").curls.txt" 2>/dev/null
done < .doc-audit/all-docs.txt
) &

# Background task 6: Find documentation without source references
echo "[2.6] Finding undocumented sources..."
(
while IFS= read -r doc; do
    if ! grep -q "<!-- SOURCE\|<!-- Source\|Source:\|<!-- Code:" "$doc" 2>/dev/null; then
        echo "$doc" >> .doc-audit/to-update/missing-sources.txt
    fi
done < .doc-audit/all-docs.txt
) &

# Background task 7: Check for outdated version references
echo "[2.7] Checking version references..."
grep -r "Go 1\.[0-9]\+\|go1\.[0-9]\+\|version.*1\.[0-9]\+" --include="*.md" . 2>/dev/null | \
    grep -v "Go 1.24\|go 1.24" > .doc-audit/outdated/version-refs.txt &

# Background task 8: Find references to non-existent files
echo "[2.8] Finding references to non-existent files..."
(
grep -r "apps/\|pkg/\|configs/\|scripts/" --include="*.md" . 2>/dev/null | while read line; do
    file=$(echo "$line" | cut -d: -f1)
    # Extract file paths that look like source references
    echo "$line" | grep -o '[a-zA-Z0-9_/.-]*\.go\|[a-zA-Z0-9_/.-]*\.yaml\|[a-zA-Z0-9_/.-]*\.yml' | while read ref; do
        if [[ ! -f "$ref" ]] && [[ ! -f "./$ref" ]]; then
            echo "$file: Missing file reference: $ref" >> .doc-audit/broken-links/files.txt
        fi
    done
done
) &

# Background task 9: Find duplicate documentation
echo "[2.9] Finding duplicate documentation..."
(
# Create content hashes for similarity detection
for doc in $(cat .doc-audit/all-docs.txt); do
    # Create a normalized version (remove whitespace variations)
    cat "$doc" 2>/dev/null | tr -s ' \t\n' | md5sum | cut -d' ' -f1 > ".doc-audit/to-update/$(basename "$doc").hash"
done
) &

# Background task 10: Check for API endpoint documentation
echo "[2.10] Checking API endpoint documentation..."
grep -r "GET \|POST \|PUT \|DELETE \|PATCH " --include="*.md" . 2>/dev/null > .doc-audit/to-update/api-endpoints.txt &

wait
echo "[Stream 2] Documentation audit complete. Results in .doc-audit/"

# Generate audit summary
cat > .doc-audit/summary.txt << EOF
Documentation Audit Summary
===========================
Total Docs: $(wc -l < .doc-audit/all-docs.txt 2>/dev/null || echo 0)
Broken Internal Links: $(wc -l < .doc-audit/broken-links/internal.txt 2>/dev/null || echo 0)
Missing File References: $(wc -l < .doc-audit/broken-links/files.txt 2>/dev/null || echo 0)
Placeholders Found: $(wc -l < .doc-audit/outdated/placeholders.txt 2>/dev/null || echo 0)
Future Features: $(wc -l < .doc-audit/outdated/future-features.txt 2>/dev/null || echo 0)
Missing Source Refs: $(wc -l < .doc-audit/to-update/missing-sources.txt 2>/dev/null || echo 0)
Outdated Versions: $(wc -l < .doc-audit/outdated/version-refs.txt 2>/dev/null || echo 0)
API Endpoints Documented: $(wc -l < .doc-audit/to-update/api-endpoints.txt 2>/dev/null || echo 0)
EOF