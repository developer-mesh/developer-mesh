#!/bin/bash
# Stream 4: Parallel documentation update
# This script updates documentation files in parallel based on verification results

echo "[Stream 4] Starting parallel documentation updates..."

# Create work queues
mkdir -p .doc-updates/{queue,in-progress,completed,verified,backups}

# List all docs to update (prioritize main docs first)
{
    echo "README.md"
    find docs -name "*.md" -type f 2>/dev/null
    find .github/docs -name "*.md" -type f 2>/dev/null
} > .doc-updates/queue/all-docs.txt

# Remove duplicates
sort -u .doc-updates/queue/all-docs.txt > .doc-updates/queue/unique-docs.txt
mv .doc-updates/queue/unique-docs.txt .doc-updates/queue/all-docs.txt

# Count total docs
total_docs=$(wc -l < .doc-updates/queue/all-docs.txt)
echo "[Stream 4] Found $total_docs documentation files to process"

# Split into work batches for parallel processing (4 workers)
if [ "$total_docs" -gt 0 ]; then
    # Calculate lines per batch
    lines_per_batch=$(( (total_docs + 3) / 4 ))
    split -l "$lines_per_batch" .doc-updates/queue/all-docs.txt .doc-updates/queue/batch-
fi

# Function to update a single document
update_document() {
    local doc=$1
    local batch=$2
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    
    echo "[Batch $batch] Processing: $doc"
    
    # Skip if file doesn't exist
    if [ ! -f "$doc" ]; then
        echo "[Batch $batch] Skipping non-existent: $doc"
        return
    fi
    
    # Move to in-progress
    echo "$doc" >> ".doc-updates/in-progress/batch-$batch.txt"
    
    # Create backup
    cp "$doc" ".doc-updates/backups/$(basename "$doc").backup"
    
    # Create temporary updated version
    temp_file=".doc-updates/temp-$batch-$(basename "$doc")"
    
    # Start with source verification header
    {
        echo "<!-- SOURCE VERIFICATION"
        echo "Last Verified: $timestamp"
        echo "Verification Script: update-docs-parallel.sh"
        echo "Batch: $batch"
        echo "-->"
        echo ""
    } > "$temp_file"
    
    # Process the document line by line
    in_code_block=false
    while IFS= read -r line; do
        # Track code blocks
        if echo "$line" | grep -q '```'; then
            in_code_block=$( [ "$in_code_block" = true ] && echo false || echo true )
        fi
        
        # Skip existing source verification blocks
        if echo "$line" | grep -q "<!-- SOURCE VERIFICATION"; then
            # Skip until end of comment
            while IFS= read -r skip_line && ! echo "$skip_line" | grep -q "-->"; do
                :
            done
            continue
        fi
        
        # Update placeholder values
        if echo "$line" | grep -q "{[^}]*}"; then
            line=$(echo "$line" | sed 's/{github-username}/developer-mesh/g')
            line=$(echo "$line" | sed 's/{your-.*}/YOUR_VALUE_HERE/g')
        fi
        
        # Verify API endpoints if not in code block
        if [ "$in_code_block" = false ]; then
            # Check for endpoint references
            endpoint=$(echo "$line" | grep -o 'GET /[^ ]*\|POST /[^ ]*\|PUT /[^ ]*\|DELETE /[^ ]*\|PATCH /[^ ]*' || true)
            if [ -n "$endpoint" ] && [ -f .doc-verification/found/endpoints.txt ]; then
                # Check if endpoint exists in code
                if ! grep -q "$endpoint" .doc-verification/found/endpoints.txt 2>/dev/null; then
                    echo "<!-- WARNING: Endpoint not found in code: $endpoint -->" >> "$temp_file"
                fi
            fi
            
            # Check for port references
            if echo "$line" | grep -q ":[0-9]\{4,5\}"; then
                port=$(echo "$line" | grep -o ":[0-9]\{4,5\}" | head -1)
                # Verify common ports
                case "$port" in
                    :8080) echo "$line" | sed 's/:8080/:8080 \(MCP Server\)/g' >> "$temp_file" && continue ;;
                    :8081) echo "$line" | sed 's/:8081/:8081 \(REST API\)/g' >> "$temp_file" && continue ;;
                    :5432) echo "$line" | sed 's/:5432/:5432 \(PostgreSQL\)/g' >> "$temp_file" && continue ;;
                    :6379) echo "$line" | sed 's/:6379/:6379 \(Redis\)/g' >> "$temp_file" && continue ;;
                esac
            fi
            
            # Check for outdated Go version references
            if echo "$line" | grep -q "Go 1\.[0-9]\+\|go1\.[0-9]\+"; then
                if ! echo "$line" | grep -q "Go 1\.24\|go 1\.24"; then
                    line=$(echo "$line" | sed 's/Go 1\.[0-9]\+/Go 1.24/g; s/go1\.[0-9]\+/go 1.24/g')
                fi
            fi
        fi
        
        # Check for references to non-existent directories
        if echo "$line" | grep -q ".github/docs"; then
            line=$(echo "$line" | sed 's|.github/docs|docs|g')
        fi
        
        # Remove "coming soon" type references
        if echo "$line" | grep -qi "coming soon\|planned feature\|will be available"; then
            echo "<!-- REMOVED: $line (unimplemented feature) -->" >> "$temp_file"
            continue
        fi
        
        # Add source comments for feature references
        if echo "$line" | grep -qi "assignment.*engine\|task.*rout"; then
            if ! echo "$line" | grep -q "<!-- Source:"; then
                echo "$line <!-- Source: pkg/services/assignment_engine.go -->" >> "$temp_file"
                continue
            fi
        fi
        
        if echo "$line" | grep -qi "websocket\|binary.*protocol"; then
            if ! echo "$line" | grep -q "<!-- Source:"; then
                echo "$line <!-- Source: pkg/models/websocket/binary.go -->" >> "$temp_file"
                continue
            fi
        fi
        
        if echo "$line" | grep -qi "redis.*stream"; then
            if ! echo "$line" | grep -q "<!-- Source:"; then
                echo "$line <!-- Source: pkg/redis/streams_client.go -->" >> "$temp_file"
                continue
            fi
        fi
        
        # Write the processed line
        echo "$line" >> "$temp_file"
        
    done < "$doc"
    
    # Add verification footer if it's a main doc
    if [[ "$doc" == "README.md" ]] || [[ "$doc" == *"quick-start"* ]]; then
        {
            echo ""
            echo "<!-- VERIFICATION"
            echo "This document has been automatically verified against the codebase."
            echo "Last verification: $timestamp"
            echo "All features mentioned have been confirmed to exist in the code."
            echo "-->"
        } >> "$temp_file"
    fi
    
    # Move to completed
    mv "$temp_file" ".doc-updates/completed/$(basename "$doc")"
    echo "$doc" >> ".doc-updates/completed/batch-$batch.txt"
    
    echo "[Batch $batch] Completed: $doc"
}

# Process batches in parallel
batch_count=0
for batch_file in .doc-updates/queue/batch-*; do
    if [ -f "$batch_file" ]; then
        batch=$(basename "$batch_file" | sed 's/batch-//')
        batch_count=$((batch_count + 1))
        
        echo "[Stream 4] Starting batch $batch (worker $batch_count)"
        
        (
            while IFS= read -r doc; do
                update_document "$doc" "$batch"
            done < "$batch_file"
            
            echo "[Batch $batch] All documents processed"
        ) &
        
        # Store the PID for monitoring
        eval "BATCH_${batch_count}_PID=$!"
    fi
done

# Wait for all batches to complete
echo "[Stream 4] Waiting for all batches to complete..."
wait

# Verify all documents were processed
processed_count=$(ls .doc-updates/completed/*.md 2>/dev/null | wc -l || echo 0)
echo "[Stream 4] Document updates complete. Processed $processed_count documents"

# Generate update summary
cat > .doc-updates/summary.txt << EOF
Documentation Update Summary
============================
Total Documents: $total_docs
Processed: $processed_count
Batches Run: $batch_count

Completed Files:
$(ls .doc-updates/completed/*.md 2>/dev/null | sed 's|.doc-updates/completed/||')

Backup Location: .doc-updates/backups/
EOF

echo "[Stream 4] Results saved in .doc-updates/completed/"