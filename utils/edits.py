#!/usr/bin/env python3
"""
Go file editor helper for the paniniclaw project.
Use this for safe, reversible code changes.

Usage:
    python3 utils/edits.py --file path/to/file.go --replace "old" "new"
    
Example:
    python3 utils/edits.py --file integrations/telegram.go --replace 'update.Message.Text' 'buildMessageContent(update.Message)'
"""

import sys
import argparse
import re

def edit_go_file(filepath, replacements):
    """
    Edit a Go file with multiple replacements.
    Returns True if any changes were made.
    """
    try:
        with open(filepath, 'r') as f:
            content = f.read()
        
        original_content = content
        changed = False
        
        for old, new in replacements.items():
            if old in content:
                content = content.replace(old, new)
                print(f"✓ Replaced: {repr(old[:50])} -> ...")
                changed = True
            else:
                print(f"⚠️  Pattern not found: {repr(old[:50])}...")
        
        if changed:
            with open(filepath, 'w') as f:
                f.write(content)
            print(f"✓ Saved {filepath} ({len(content)} bytes)")
            return True
        else:
            print("ℹ️  No changes needed")
            return False
            
    except Exception as e:
        print(f"❌ Error: {e}", file=sys.stderr)
        return False

def edit_go_file_regex(filepath, pattern, replacement):
    """
    Edit a Go file with a regex replacement.
    """
    try:
        with open(filepath, 'r') as f:
            content = f.read()
        
        original = content
        new_content = re.sub(pattern, replacement, content, flags=re.MULTILINE | re.DOTALL)
        
        if new_content != original:
            with open(filepath, 'w') as f:
                f.write(new_content)
            print(f"✓ Regex replacement applied to {filepath}")
            return True
        else:
            print("ℹ️  No regex matches found")
            return False
            
    except Exception as e:
        print(f"❌ Error: {e}", file=sys.stderr)
        return False

def main():
    parser = argparse.ArgumentParser(description='Edit Go files safely')
    parser.add_argument('--file', '-f', required=True, help='Go file to edit')
    
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--replace', '-r', nargs=2, metavar=('OLD', 'NEW'), help='String replacement')
    group.add_argument('--regex', '-x', nargs=2, metavar=('PATTERN', 'REPLACEMENT'), help='Regex replacement')
    
    args = parser.parse_args()
    
    if args.replace:
        old, new = args.replace
        edit_go_file(args.file, {old: new})
    elif args.regex:
        pattern, replacement = args.regex
        edit_go_file_regex(args.file, pattern, replacement)

if __name__ == '__main__':
    main()
