#!/usr/bin/env python3
"""
Fix common markdown linting issues in generated documentation.

This fixes:
- MD032: Blank lines around lists
- MD040: Language specification for fenced code blocks
- MD022: Blank lines around headings
- MD031: Blank lines around fenced code blocks
"""

import re
import sys
from pathlib import Path

def fix_markdown_file(file_path):
    """Fix markdown linting issues in a single file."""
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    original_content = content

    # Fix MD040: Add 'text' language to code blocks without language
    # Pattern: ```\n (not preceded by a language identifier)
    content = re.sub(r'```\n(?!```)', '```text\n', content)

    # Fix issues line by line
    lines = content.split('\n')
    new_lines = []

    for i, line in enumerate(lines):
        prev_line = new_lines[-1] if new_lines else ''

        # Fix MD032: Add blank line before list items if not present
        if (re.match(r'^[\*\-\+]\s', line.lstrip()) or re.match(r'^\d+\.\s', line.lstrip())):
            # If previous line exists, isn't blank, and isn't a list item
            if (prev_line and prev_line.strip() and
                not re.match(r'^[\*\-\+]\s', prev_line.lstrip()) and
                not re.match(r'^\d+\.\s', prev_line.lstrip()) and
                not prev_line.strip().startswith('#')):
                new_lines.append('')

        # Fix MD031: Add blank line before code fences if not present
        if line.strip().startswith('```') and i > 0:
            if prev_line.strip() and not prev_line.strip().startswith('```'):
                # Insert blank line if not already there
                if new_lines and new_lines[-1].strip():
                    new_lines.append('')

        # Fix MD022: Add blank line before headings if not present
        if line.strip().startswith('#') and i > 0:
            if prev_line.strip() and not prev_line.strip().startswith('#'):
                if new_lines and new_lines[-1].strip():
                    new_lines.append('')

        new_lines.append(line)

    content = '\n'.join(new_lines)

    # Return True if changes were made
    if content != original_content:
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return True
    return False

def main():
    """Find and fix all markdown files."""
    # Find all markdown files
    md_files = []
    for pattern in ['*.md', '**/*.md']:
        md_files.extend(Path('.').glob(pattern))

    # Filter out excluded paths
    excluded = {'node_modules', '.git', 'vendor'}
    md_files = [f for f in md_files if not any(ex in f.parts for ex in excluded)]

    fixed_count = 0
    total_count = len(md_files)

    print(f"\033[0;32mFixing markdown linting issues in {total_count} files...\033[0m\n")

    for file_path in sorted(md_files):
        print(f"\033[1;33mProcessing: {file_path}\033[0m", end=' ')
        try:
            if fix_markdown_file(file_path):
                fixed_count += 1
                print("\033[0;32m✓ Fixed\033[0m")
            else:
                print("No changes needed")
        except Exception as e:
            print(f"\033[0;31m✗ Error: {e}\033[0m")

    print()
    print("\033[0;32m" + "=" * 40 + "\033[0m")
    print(f"\033[0;32mFixed {fixed_count} out of {total_count} files\033[0m")
    print("\033[0;32m" + "=" * 40 + "\033[0m")

if __name__ == '__main__':
    main()
