#!/bin/bash
# Master Orchestration Script for Parallel Documentation Update
# This script coordinates all parallel streams and monitors progress

echo "========================================="
echo "Developer Mesh Documentation Update System"
echo "========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Start time tracking
START_TIME=$(date +%s)
START_DATE=$(date +"%Y-%m-%d %H:%M:%S")

# Clean previous runs
echo -e "${BLUE}[MAIN] Cleaning previous run artifacts...${NC}"
rm -rf .doc-{verification,audit,testing,updates} 2>/dev/null
rm -f stream*.log 2>/dev/null

# Make all scripts executable
chmod +x verify-codebase.sh audit-docs.sh test-commands.sh update-docs-parallel.sh 2>/dev/null

# Launch all streams in background
echo -e "${BLUE}[MAIN] Launching parallel work streams...${NC}"
echo ""

# Stream 1: Code Verification (Background)
echo -e "${YELLOW}[MAIN] Starting Stream 1: Code Verification${NC}"
./verify-codebase.sh > stream1.log 2>&1 &
STREAM1_PID=$!
echo -e "       PID: $STREAM1_PID - Logging to: stream1.log"

# Stream 2: Documentation Audit (Background)
echo -e "${YELLOW}[MAIN] Starting Stream 2: Documentation Audit${NC}"
./audit-docs.sh > stream2.log 2>&1 &
STREAM2_PID=$!
echo -e "       PID: $STREAM2_PID - Logging to: stream2.log"

# Stream 3: Test Execution (Background)
echo -e "${YELLOW}[MAIN] Starting Stream 3: Test Execution${NC}"
./test-commands.sh > stream3.log 2>&1 &
STREAM3_PID=$!
echo -e "       PID: $STREAM3_PID - Logging to: stream3.log"

# Wait for initial verification data (give streams time to create initial data)
echo ""
echo -e "${BLUE}[MAIN] Waiting for initial verification data...${NC}"
sleep 5

# Stream 4: Document Updates (Parallel processing)
echo -e "${YELLOW}[MAIN] Starting Stream 4: Parallel Document Updates${NC}"
./update-docs-parallel.sh > stream4.log 2>&1 &
STREAM4_PID=$!
echo -e "       PID: $STREAM4_PID - Logging to: stream4.log"

echo ""
echo -e "${BLUE}[MAIN] All streams launched. Monitoring progress...${NC}"
echo ""

# Function to check if process is running
is_running() {
    ps -p $1 > /dev/null 2>&1
}

# Function to get stream status
get_stream_status() {
    local pid=$1
    local name=$2
    if is_running $pid; then
        echo -e "${YELLOW}Running${NC}"
    else
        echo -e "${GREEN}Complete${NC}"
    fi
}

# Monitor progress
monitor_count=0
while true; do
    # Clear screen only after first iteration to show initial output
    if [ $monitor_count -gt 0 ]; then
        clear
    fi
    
    echo "========================================="
    echo "Documentation Update Progress Monitor"
    echo "========================================="
    echo -e "Started: ${CYAN}$START_DATE${NC}"
    echo -e "Elapsed: ${CYAN}$(($(date +%s) - START_TIME))${NC} seconds"
    echo ""
    
    # Stream status
    echo -e "${BLUE}Stream Status:${NC}"
    echo -e "  Stream 1 (Code Verification):    $(get_stream_status $STREAM1_PID 'Code Verification')"
    echo -e "  Stream 2 (Documentation Audit):  $(get_stream_status $STREAM2_PID 'Documentation Audit')"
    echo -e "  Stream 3 (Test Execution):       $(get_stream_status $STREAM3_PID 'Test Execution')"
    echo -e "  Stream 4 (Document Updates):     $(get_stream_status $STREAM4_PID 'Document Updates')"
    echo ""
    
    # Progress metrics
    echo -e "${BLUE}Progress Metrics:${NC}"
    
    # Stream 1 metrics
    if [ -d .doc-verification/found ]; then
        echo -e "${CYAN}  Code Verification:${NC}"
        echo "    - Handlers found:     $(wc -l < .doc-verification/found/handlers.txt 2>/dev/null || echo 0)"
        echo "    - Endpoints found:    $(wc -l < .doc-verification/found/endpoints.txt 2>/dev/null || echo 0)"
        echo "    - Make targets:       $(wc -l < .doc-verification/found/make-targets.txt 2>/dev/null || echo 0)"
        echo "    - Docker services:    $(wc -l < .doc-verification/found/docker-services.txt 2>/dev/null || echo 0)"
        echo "    - TODOs found:        $(wc -l < .doc-verification/todos/todo-list.txt 2>/dev/null || echo 0)"
    fi
    
    # Stream 2 metrics
    if [ -d .doc-audit ]; then
        echo -e "${CYAN}  Documentation Audit:${NC}"
        echo "    - Docs scanned:       $(wc -l < .doc-audit/all-docs.txt 2>/dev/null || echo 0)"
        echo "    - Broken links:       $(wc -l < .doc-audit/broken-links/internal.txt 2>/dev/null || echo 0)"
        echo "    - Placeholders:       $(wc -l < .doc-audit/outdated/placeholders.txt 2>/dev/null || echo 0)"
        echo "    - Future features:    $(wc -l < .doc-audit/outdated/future-features.txt 2>/dev/null || echo 0)"
    fi
    
    # Stream 3 metrics
    if [ -d .doc-testing ]; then
        echo -e "${CYAN}  Test Execution:${NC}"
        echo "    - Tests passed:       $(find .doc-testing/passed -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)"
        echo "    - Tests failed:       $(find .doc-testing/failed -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)"
        echo "    - Tests skipped:      $(find .doc-testing/skipped -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)"
    fi
    
    # Stream 4 metrics
    if [ -d .doc-updates ]; then
        echo -e "${CYAN}  Document Updates:${NC}"
        echo "    - Docs queued:        $(wc -l < .doc-updates/queue/all-docs.txt 2>/dev/null || echo 0)"
        echo "    - Docs completed:     $(ls .doc-updates/completed/*.md 2>/dev/null | wc -l || echo 0)"
        echo "    - Backups created:    $(ls .doc-updates/backups/*.backup 2>/dev/null | wc -l || echo 0)"
    fi
    
    echo ""
    
    # Check if all streams are complete
    all_complete=true
    for pid in $STREAM1_PID $STREAM2_PID $STREAM3_PID $STREAM4_PID; do
        if is_running $pid; then
            all_complete=false
            break
        fi
    done
    
    if [ "$all_complete" = true ]; then
        echo -e "${GREEN}All streams complete!${NC}"
        break
    fi
    
    # Show hint for logs
    if [ $monitor_count -eq 0 ]; then
        echo -e "${BLUE}Tip: Check individual stream logs for detailed progress:${NC}"
        echo "  tail -f stream1.log  # Code verification details"
        echo "  tail -f stream2.log  # Documentation audit details"
        echo "  tail -f stream3.log  # Test execution details"
        echo "  tail -f stream4.log  # Document update details"
    fi
    
    monitor_count=$((monitor_count + 1))
    sleep 3
done

# Generate final report
echo ""
echo -e "${BLUE}[MAIN] Generating final report...${NC}"

END_TIME=$(date +%s)
END_DATE=$(date +"%Y-%m-%d %H:%M:%S")
DURATION=$((END_TIME - START_TIME))

cat > PARALLEL_UPDATE_REPORT.md << EOF
# Parallel Documentation Update Report

## Execution Summary
- **Start Time:** $START_DATE
- **End Time:** $END_DATE
- **Duration:** $DURATION seconds ($(($DURATION / 60)) minutes)
- **Parallel Streams:** 4

## Stream 1: Code Verification Results
$([ -f .doc-verification/summary.txt ] && cat .doc-verification/summary.txt || echo "No summary available")

### Key Findings
- **Handlers Found:** $(wc -l < .doc-verification/found/handlers.txt 2>/dev/null || echo 0)
- **Endpoints Found:** $(wc -l < .doc-verification/found/endpoints.txt 2>/dev/null || echo 0)
- **TODOs/FIXMEs:** $(wc -l < .doc-verification/todos/todo-list.txt 2>/dev/null || echo 0)
- **Environment Variables:** $(wc -l < .doc-verification/found/env-vars.txt 2>/dev/null || echo 0)

## Stream 2: Documentation Audit Results
$([ -f .doc-audit/summary.txt ] && cat .doc-audit/summary.txt || echo "No summary available")

### Issues Found
- **Broken Internal Links:** $(wc -l < .doc-audit/broken-links/internal.txt 2>/dev/null || echo 0)
- **Placeholder Values:** $(wc -l < .doc-audit/outdated/placeholders.txt 2>/dev/null || echo 0)
- **Future/Planned Features:** $(wc -l < .doc-audit/outdated/future-features.txt 2>/dev/null || echo 0)
- **Missing Source References:** $(wc -l < .doc-audit/to-update/missing-sources.txt 2>/dev/null || echo 0)

## Stream 3: Test Execution Results
$([ -f .doc-testing/summary.txt ] && cat .doc-testing/summary.txt || echo "No summary available")

### Test Statistics
- **Total Tests Passed:** $(find .doc-testing/passed -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)
- **Total Tests Failed:** $(find .doc-testing/failed -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)
- **Total Tests Skipped:** $(find .doc-testing/skipped -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}' || echo 0)

## Stream 4: Documentation Updates
$([ -f .doc-updates/summary.txt ] && cat .doc-updates/summary.txt || echo "No summary available")

### Update Statistics
- **Documents Processed:** $(ls .doc-updates/completed/*.md 2>/dev/null | wc -l || echo 0)
- **Backups Created:** $(ls .doc-updates/backups/*.backup 2>/dev/null | wc -l || echo 0)

## Action Items

### High Priority
$([ -f .doc-audit/broken-links/internal.txt ] && [ -s .doc-audit/broken-links/internal.txt ] && echo "1. Fix broken internal links (see .doc-audit/broken-links/internal.txt)" || echo "1. ✓ No broken links found")
$([ -f .doc-audit/outdated/placeholders.txt ] && [ -s .doc-audit/outdated/placeholders.txt ] && echo "2. Update placeholder values (see .doc-audit/outdated/placeholders.txt)" || echo "2. ✓ No placeholders found")
$([ -f .doc-testing/failed/make-targets.txt ] && [ -s .doc-testing/failed/make-targets.txt ] && echo "3. Fix failing make targets (see .doc-testing/failed/make-targets.txt)" || echo "3. ✓ All make targets valid")

### Medium Priority
4. Review and remove future/planned features from documentation
5. Add source references to undocumented features
6. Update outdated version references

### Low Priority
7. Review TODO items in code
8. Update code comments for better documentation
9. Add missing test coverage

## Files and Directories Created

### Verification Results
- \`.doc-verification/\` - Code analysis results
- \`.doc-audit/\` - Documentation audit results
- \`.doc-testing/\` - Test execution results
- \`.doc-updates/\` - Updated documentation files

### Log Files
- \`stream1.log\` - Code verification log
- \`stream2.log\` - Documentation audit log
- \`stream3.log\` - Test execution log
- \`stream4.log\` - Document update log

## Next Steps

1. **Review Updates:** Check \`.doc-updates/completed/\` for all updated documentation
2. **Apply Updates:** Run \`./apply-updates.sh\` to apply all changes
3. **Commit Changes:** After review, commit the updated documentation
4. **Run Verification:** Use \`./verify-docs.sh\` to verify all changes

## Performance Metrics

- **Parallel Efficiency:** 4 streams running concurrently
- **Documents/Second:** $(echo "scale=2; $(ls .doc-updates/completed/*.md 2>/dev/null | wc -l) / $DURATION" | bc 2>/dev/null || echo "N/A")
- **Tests/Second:** $(echo "scale=2; $(find .doc-testing/passed -type f -exec wc -l {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}') / $DURATION" | bc 2>/dev/null || echo "N/A")

---
*Report generated on $END_DATE*
EOF

# Create apply script
cat > apply-updates.sh << 'EOF'
#!/bin/bash
# Apply all documentation updates from the parallel update process

echo "========================================="
echo "Applying Documentation Updates"
echo "========================================="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

if [ ! -d .doc-updates/completed ]; then
    echo -e "${RED}Error: No completed updates found in .doc-updates/completed/${NC}"
    exit 1
fi

updated_count=0
skipped_count=0

echo -e "${YELLOW}Applying updates...${NC}"
echo ""

# Apply updates to original locations
for updated in .doc-updates/completed/*.md; do
    if [ -f "$updated" ]; then
        filename=$(basename "$updated")
        
        # Find original file location
        original=""
        if [ "$filename" = "README.md" ]; then
            original="README.md"
        elif [ -f "docs/$filename" ]; then
            original="docs/$filename"
        elif [ -f ".github/docs/$filename" ]; then
            original=".github/docs/$filename"
        else
            # Search for the file
            found=$(find docs .github/docs -name "$filename" -type f 2>/dev/null | head -1)
            if [ -n "$found" ]; then
                original="$found"
            fi
        fi
        
        if [ -n "$original" ] && [ -f "$original" ]; then
            cp "$updated" "$original"
            echo -e "${GREEN}✓${NC} Updated: $original"
            updated_count=$((updated_count + 1))
        else
            echo -e "${YELLOW}⚠${NC} Skipped: $filename (original not found)"
            skipped_count=$((skipped_count + 1))
        fi
    fi
done

echo ""
echo "========================================="
echo -e "${GREEN}Documentation updates applied!${NC}"
echo "- Updated: $updated_count files"
echo "- Skipped: $skipped_count files"
echo ""
echo "Next steps:"
echo "1. Review the changes with: git diff"
echo "2. Run verification: ./verify-docs.sh"
echo "3. Commit changes: git add -A && git commit -m 'docs: update documentation with verified content'"
echo "========================================="
EOF

chmod +x apply-updates.sh

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}Documentation Update Complete!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo -e "${BLUE}Reports and Results:${NC}"
echo "  - Full Report: PARALLEL_UPDATE_REPORT.md"
echo "  - Stream Summaries:"
echo "    - .doc-verification/summary.txt"
echo "    - .doc-audit/summary.txt"
echo "    - .doc-testing/summary.txt"
echo "    - .doc-updates/summary.txt"
echo ""
echo -e "${BLUE}To apply all updates:${NC}"
echo "  ./apply-updates.sh"
echo ""
echo -e "${BLUE}To review specific issues:${NC}"
echo "  - Broken links: cat .doc-audit/broken-links/internal.txt"
echo "  - Placeholders: cat .doc-audit/outdated/placeholders.txt"
echo "  - Failed tests: ls .doc-testing/failed/"
echo ""
echo -e "${GREEN}Total execution time: $DURATION seconds${NC}"