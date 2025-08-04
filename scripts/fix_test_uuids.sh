#!/bin/bash

# Script to migrate hardcoded test IDs to UUID format
# This helps maintain consistency in test data across the codebase

echo "🔧 Fixing hardcoded test IDs in Go test files..."

# Define common replacements
declare -A REPLACEMENTS=(
    # Common tenant IDs
    ["tenant-123"]="11111111-1111-1111-1111-111111111111"
    ["tenant-456"]="22222222-2222-2222-2222-222222222222"
    ["tenant-789"]="33333333-3333-3333-3333-333333333333"
    ["test-tenant"]="11111111-1111-1111-1111-111111111111"
    ["default-tenant"]="00000000-0000-0000-0000-000000000001"
    
    # Common user IDs
    ["user-123"]="22222222-2222-2222-2222-222222222222"
    ["user-456"]="44444444-4444-4444-4444-444444444444"
    ["test-user"]="22222222-2222-2222-2222-222222222222"
    ["default-user"]="00000000-0000-0000-0000-000000000001"
    
    # Common agent/model IDs
    ["agent-123"]="55555555-5555-5555-5555-555555555555"
    ["model-123"]="66666666-6666-6666-6666-666666666666"
    ["tool-123"]="77777777-7777-7777-7777-777777777777"
)

# Function to replace IDs in a file
replace_ids() {
    local file=$1
    local modified=false
    
    for old_id in "${!REPLACEMENTS[@]}"; do
        new_id="${REPLACEMENTS[$old_id]}"
        
        # Check if the file contains the old ID
        if grep -q "$old_id" "$file"; then
            # Use sed to replace, handling different contexts
            # Replace in string literals
            sed -i.bak "s/\"$old_id\"/\"$new_id\"/g" "$file"
            sed -i.bak "s/'$old_id'/'$new_id'/g" "$file"
            
            # Replace in variable assignments
            sed -i.bak "s/= \"$old_id\"/= \"$new_id\"/g" "$file"
            
            # Replace in function calls
            sed -i.bak "s/($old_id)/($new_id)/g" "$file"
            
            modified=true
        fi
    done
    
    # Clean up backup files
    rm -f "${file}.bak"
    
    if [ "$modified" = true ]; then
        echo "✓ Fixed: $file"
    fi
}

# Find all Go test files
echo "Searching for Go test files..."
test_files=$(find . -name "*_test.go" -type f | grep -v vendor | grep -v .git)

# Process each file
for file in $test_files; do
    replace_ids "$file"
done

echo ""
echo "🔍 Checking for remaining non-UUID test IDs..."

# Look for potential missed IDs (common patterns)
remaining=$(grep -r -E '(tenant|user|agent|model|tool)-[0-9]+' --include="*_test.go" . | grep -v vendor | grep -v .git | grep -v "uuid.MustParse" || true)

if [ -n "$remaining" ]; then
    echo "⚠️  Found potential non-UUID test IDs that may need manual review:"
    echo "$remaining"
    echo ""
    echo "Consider using testutil.TestTenantID() and testutil.TestUserID() helpers instead."
else
    echo "✅ No obvious non-UUID test IDs found!"
fi

echo ""
echo "📋 Next steps:"
echo "1. Run 'make test' to verify all tests still pass"
echo "2. Review any test failures and update as needed"
echo "3. Consider using pkg/testutil UUID helpers for new tests"