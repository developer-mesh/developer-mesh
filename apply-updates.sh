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
