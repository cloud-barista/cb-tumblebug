# CB-Tumblebug Documentation Tools

This directory contains tools for generating and analyzing CB-Tumblebug documentation.

## Available Tools

### swagger-diff-analyzer.py

A comprehensive API change analyzer that compares Swagger/OpenAPI specifications between different versions of CB-Tumblebug.

**Features:**
- Local git repository analysis (preferred method)
- GitHub repository fallback
- Comprehensive diff generation
- Breaking change detection
- Markdown report generation
- Zero external dependencies (uses only Python standard library)

**Usage:**
```bash
# Basic usage
python3 swagger-diff-analyzer.py v0.11.8 main

# With custom output file
python3 swagger-diff-analyzer.py v0.11.8 v0.11.10 -o my-report.md

# Using local swagger files
python3 swagger-diff-analyzer.py --local-old /path/to/old/swagger.json --local-new /path/to/new/swagger.json

# Verbose output
python3 swagger-diff-analyzer.py v0.11.8 main -v
```

**Dependencies:**
- Python 3.6+
- No external dependencies required (requests is optional for GitHub fallback)

**Output:**
The tool generates detailed markdown reports in the `docs/` directory with:
- Version comparison summary
- API endpoint changes (additions, removals, modifications)
- Breaking change detection
- Schema changes analysis
- Migration guidance

### requirements.txt

Minimal dependency specification for the documentation tools. Currently indicates zero required dependencies, with optional requests library for GitHub API fallback.

## Directory Structure

```
docs/docs_tools/
├── README.md                     # This file
├── swagger-diff-analyzer.py      # API change analyzer
└── requirements.txt              # Dependencies (optional)
```

## Reports Location

Generated reports are saved to the parent `docs/` directory with naming convention:
- `apidiff_{old_version}_{new_version}.md`

## Development

The tools are designed to be:
- **Self-contained**: Minimal external dependencies
- **Portable**: Work from any directory within the CB-Tumblebug repository
- **Robust**: Graceful fallback mechanisms
- **Professional**: Complete English documentation

For more information, see the individual tool documentation within each script.