#!/bin/bash

# Script to fix common markdown linting issues in generated documentation
# This fixes the main issues flagged by markdownlint:
# - MD032: Blank lines around lists
# - MD040: Language specification for fenced code blocks
# - MD022: Blank lines around headings
# - MD031: Blank lines around fenced code blocks

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Fixing markdown linting issues...${NC}"

# Find all markdown files (excluding node_modules, .git)
MD_FILES=$(find . -name "*.md" \
    -not -path "./node_modules/*" \
    -not -path "./.git/*" \
    -not -path "./vendor/*" \
    -type f)

FIXED_COUNT=0
TOTAL_COUNT=0

for file in $MD_FILES; do
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    echo -e "${YELLOW}Processing: $file${NC}"

    # Create a backup
    cp "$file" "$file.backup"

    # Fix the file using Python (more reliable than sed for multiline operations)
    python3 -c "
import re
import sys

with open('$file', 'r', encoding='utf-8') as f:
    content = f.read()

original_content = content

# Fix MD040: Add 'text' language to code blocks without language
# Pattern: ```\n (not preceded by a language identifier)
content = re.sub(r'(?<!\w)```\n', '```text\n', content)

# Fix MD032: Add blank lines around lists
# This is complex, so we'll do a simple version:
# Add blank line before list items if not present
lines = content.split('\n')
new_lines = []
prev_line = ''
for i, line in enumerate(lines):
    # Check if current line starts a list
    if re.match(r'^[\*\-\+]\s', line.lstrip()) or re.match(r'^\d+\.\s', line.lstrip()):
        # If previous line exists and isn't blank and isn't a list item
        if prev_line and prev_line.strip() and not re.match(r'^[\*\-\+]\s', prev_line.lstrip()) and not re.match(r'^\d+\.\s', prev_line.lstrip()):
            if not new_lines or new_lines[-1].strip():
                new_lines.append('')
    new_lines.append(line)
    prev_line = line

content = '\n'.join(new_lines)

# Fix MD031: Blank lines around fenced code blocks
# Add blank line before ``` if not present
lines = content.split('\n')
new_lines = []
for i, line in enumerate(lines):
    if line.strip().startswith('```') and i > 0:
        prev_line = new_lines[-1] if new_lines else ''
        if prev_line.strip():
            new_lines.append('')
    new_lines.append(line)

content = '\n'.join(new_lines)

# Fix MD022: Blank lines around headings
# Add blank line before headings if not present
lines = content.split('\n')
new_lines = []
for i, line in enumerate(lines):
    if line.strip().startswith('#') and i > 0:
        prev_line = new_lines[-1] if new_lines else ''
        if prev_line.strip() and not prev_line.strip().startswith('#'):
            new_lines.append('')
    new_lines.append(line)

content = '\n'.join(new_lines)

# Write the fixed content
if content != original_content:
    with open('$file', 'w', encoding='utf-8') as f:
        f.write(content)
    print('FIXED', file=sys.stderr)
else:
    print('NO_CHANGES', file=sys.stderr)
" 2>&1 | grep -q "FIXED" && {
        FIXED_COUNT=$((FIXED_COUNT + 1))
        echo -e "${GREEN}✓ Fixed${NC}"
        rm "$file.backup"
    } || {
        echo -e "  No changes needed"
        rm "$file.backup"
    }
done

echo ""
echo -e "${GREEN}====================================${NC}"
echo -e "${GREEN}Fixed $FIXED_COUNT out of $TOTAL_COUNT files${NC}"
echo -e "${GREEN}====================================${NC}"
