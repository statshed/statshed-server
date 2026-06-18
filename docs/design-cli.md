# StatShed CLI - Design Document

A command-line interface for interacting with the StatShed status dashboard API.

## Overview

The StatShed CLI (`statshed`) provides a robust command-line interface for submitting job statuses and querying the StatShed dashboard. It is designed to work equally well in interactive terminal sessions, CI/CD pipelines, cron jobs, and shell scripts.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Python 3.10+ |
| CLI Framework | Click |
| HTTP Client | Requests |
| Configuration | PyYAML |
| Rich Output | Rich (optional) |
| Package Manager | uv |
| Linting/Formatting | Ruff, mypy |

## Project Structure

```
cli/
├── statshed_cli/
│   ├── __init__.py
│   ├── main.py              # CLI entry point and commands
│   ├── client.py            # API client
│   ├── config.py            # Configuration file handling
│   ├── output.py            # Output formatting (plain/rich)
│   ├── errors.py            # Error handling and exit codes
│   └── completion.py        # Shell completion utilities
├── tests/
│   ├── __init__.py
│   ├── test_client.py
│   ├── test_commands.py
│   ├── test_config.py
│   └── test_output.py
├── pyproject.toml
└── README.md
```

## Command Reference

### Global Options

| Option | Short | Environment Variable | Description |
|--------|-------|---------------------|-------------|
| `--url` | `-u` | `STATSHED_URL` | StatShed API URL (default: from config or `http://localhost:7828`) |
| `--config` | `-c` | `STATSHED_CONFIG` | Path to config file |
| `--quiet` | `-q` | - | Suppress non-error output |
| `--no-color` | - | `NO_COLOR` | Disable colored output |
| `--json` | - | - | Output in JSON format (where applicable) |

### Commands

#### `submit` - Submit Job Status

```bash
statshed submit --group <name> --job <name> --status <status> [--message <msg>]
```

| Option | Short | Required | Description |
|--------|-------|----------|-------------|
| `--group` | `-g` | Yes | Group name |
| `--job` | `-j` | Yes | Job name |
| `--status` | `-s` | Yes | Status: `success`, `error`, `progress` |
| `--message` | `-m` | No | Optional status message |
| `--strict` | - | No | Exit with error code on failure (default: swallow errors) |

**Error Handling Modes:**

- **Default (lenient)**: Errors are logged (optionally to syslog) but exit code is 0. Safe for use in scripts with `set -eu`.
- **Strict (`--strict`)**: Errors produce non-zero exit codes and error output.

#### `health` - System Health Summary

```bash
statshed health [--json]
```

Returns overall system health status. Exits with code 1 if unhealthy.

#### `groups` - List Groups

```bash
statshed groups [--json]
```

Lists all groups with health summaries.

#### `jobs` - List Jobs in Group

```bash
statshed jobs <group_name> [--json]
```

Lists all jobs within a specific group.

#### `config` - Global Configuration

```bash
# View global config
statshed config

# Update global config
statshed config --progress-timeout <minutes> --staleness-timeout <hours>
```

| Option | Short | Description |
|--------|-------|-------------|
| `--progress-timeout` | `-p` | Progress timeout in minutes |
| `--staleness-timeout` | `-s` | Staleness timeout in hours |
| `--json` | - | Output as JSON |

#### `group-config` - Group-Specific Configuration

```bash
# View group config
statshed group-config <group_name>

# Update group config
statshed group-config <group_name> --progress-timeout <minutes>

# Reset to global defaults
statshed group-config <group_name> --reset-progress-timeout --reset-staleness-timeout
```

| Option | Short | Description |
|--------|-------|-------------|
| `--progress-timeout` | `-p` | Override progress timeout (minutes) |
| `--staleness-timeout` | `-s` | Override staleness timeout (hours) |
| `--reset-progress-timeout` | - | Reset to global default |
| `--reset-staleness-timeout` | - | Reset to global default |
| `--json` | - | Output as JSON |

#### `completion` - Shell Completion

```bash
# Generate completion script
statshed completion bash > ~/.local/share/bash-completion/completions/statshed
statshed completion zsh > ~/.zfunc/_statshed
statshed completion fish > ~/.config/fish/completions/statshed.fish
```

## Configuration File

The CLI reads configuration from the following locations (in order of precedence):

1. Path specified via `--config` or `STATSHED_CONFIG`
2. `./statshed.yaml` (current directory)
3. `~/.config/statshed/statshed.yaml`
4. `/etc/statshed/statshed.yaml`

### Configuration Schema

```yaml
# StatShed CLI Configuration

# API server URL
url: http://localhost:7828

# Default output format: "table" or "json"
output_format: table

# Enable colored/rich output (true/false/auto)
# "auto" enables color when stdout is a TTY
color: auto

# Submit command behavior
submit:
  # Log errors to syslog instead of stderr
  syslog: false

  # Syslog facility (if syslog is enabled)
  syslog_facility: user

  # Default to strict mode (exit with error codes)
  strict: false

# Request timeout in seconds
timeout: 10
```

## Exit Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | SUCCESS | Command completed successfully |
| 1 | ERROR_UNHEALTHY | Health check returned unhealthy status |
| 2 | ERROR_API | API returned an error response |
| 3 | ERROR_CONNECTION | Could not connect to the server |
| 4 | ERROR_TIMEOUT | Request timed out |
| 5 | ERROR_CONFIG | Configuration file error |
| 10 | ERROR_INVALID_ARGS | Invalid command arguments |
| 11 | ERROR_NOT_FOUND | Resource not found (group, job) |

**Note:** In submit command's default (lenient) mode, all errors result in exit code 0. Use `--strict` to enable error exit codes.

## Output Formatting

### Plain Mode (default when Rich not installed or `--no-color`)

```
$ statshed health
System Health: ✅ HEALTHY
Total Jobs: 10
  Healthy: 8
  Unhealthy: 1
  In Progress: 1
```

### Rich Mode (when Rich is installed and TTY detected)

- Colored status indicators (green/red/yellow/blue)
- Formatted tables with borders
- Progress indicators for long operations
- Styled error messages

### JSON Mode (`--json`)

```json
{
  "status": "healthy",
  "total_jobs": 10,
  "healthy": 8,
  "unhealthy": 1,
  "in_progress": 1
}
```

---

## Implementation Phases

### Phase 1: Project Restructuring and Configuration

Refactor the existing CLI to support the new architecture and add configuration file support.

#### Project Setup

- [ ] Create new module structure (`client.py`, `config.py`, `output.py`, `errors.py`)
- [ ] Move `ApiClient` class to `client.py`
- [ ] Add PyYAML dependency to `pyproject.toml`
- [ ] Update package metadata in `pyproject.toml`

#### Configuration File Support

- [ ] Implement config file discovery (check paths in order of precedence)
- [ ] Implement YAML config file parser with schema validation
- [ ] Add `--config` global option to specify config file path
- [ ] Support `STATSHED_CONFIG` environment variable
- [ ] Merge config sources: defaults < config file < env vars < CLI args
- [ ] Add helpful error messages for invalid config files

#### Error Handling Foundation

- [ ] Define exit code constants in `errors.py`
- [ ] Create custom exception classes for different error types
- [ ] Implement error-to-exit-code mapping
- [ ] Add `--quiet` global option to suppress non-error output

---

### Phase 2: Enhanced Commands and Error Modes

Add group-level configuration command and implement the dual error handling modes.

#### Group Configuration Command

- [ ] Add `get_group_config()` method to `ApiClient`
- [ ] Add `update_group_config()` method to `ApiClient`
- [ ] Implement `group-config` command with view mode
- [ ] Implement `group-config` command with update mode
- [ ] Add `--reset-progress-timeout` and `--reset-staleness-timeout` options
- [ ] Add JSON output support for `group-config`

#### Submit Command Error Modes

- [ ] Add `--strict` flag to submit command
- [ ] Implement lenient mode (default): catch errors, log, exit 0
- [ ] Implement strict mode: propagate errors with exit codes
- [ ] Add `submit.strict` config file option
- [ ] Add syslog support for error logging
- [ ] Add `submit.syslog` and `submit.syslog_facility` config options
- [ ] Ensure other commands (health, groups, jobs, config) always use strict error handling

#### API Client Improvements

- [ ] Add configurable request timeout
- [ ] Add `timeout` config file option
- [ ] Improve error messages with more context
- [ ] Add retry logic for transient failures (optional, configurable)

---

### Phase 3: Rich Output and Shell Completion

Enhance user experience with optional rich terminal output and shell completion.

#### Rich Terminal Output

- [ ] Add Rich as optional dependency (`pip install statshed-cli[rich]`)
- [ ] Create `output.py` module with output abstraction
- [ ] Implement `PlainFormatter` class for basic output
- [ ] Implement `RichFormatter` class for styled output
- [ ] Auto-detect Rich availability and TTY status
- [ ] Add `--no-color` global option
- [ ] Add `color` config file option (true/false/auto)
- [ ] Style health status with colors (green=healthy, red=error, yellow=progress)
- [ ] Format group/job listings as styled tables
- [ ] Style error messages with red highlighting

#### Shell Completion

- [ ] Implement `completion` command
- [ ] Generate Bash completion script
- [ ] Generate Zsh completion script
- [ ] Generate Fish completion script
- [ ] Add dynamic completion for group names (queries API)
- [ ] Add dynamic completion for job names within groups
- [ ] Add completion for status values (success, error, progress)
- [ ] Document completion installation in README

---

### Phase 4: Testing and Documentation

Comprehensive test coverage and documentation.

#### Unit Tests

- [ ] Test `ApiClient` methods with mocked responses
- [ ] Test config file parsing and merging
- [ ] Test config file discovery logic
- [ ] Test error handling and exit codes
- [ ] Test output formatters (plain and rich)
- [ ] Test submit command lenient vs strict modes
- [ ] Test group-config command
- [ ] Test shell completion generation

#### Integration Tests

- [ ] Test CLI against running backend (pytest fixtures)
- [ ] Test submit command end-to-end
- [ ] Test health/groups/jobs queries
- [ ] Test config and group-config commands
- [ ] Test error scenarios (connection refused, timeouts, 404s)

#### Documentation

- [ ] Update CLI README with full command reference
- [ ] Document configuration file format and options
- [ ] Add examples for common use cases:
  - [ ] CI/CD pipeline integration
  - [ ] Cron job status reporting
  - [ ] Interactive terminal usage
  - [ ] Script integration with `set -eu`
- [ ] Document shell completion installation for each shell
- [ ] Add troubleshooting section

---

### Phase 5: Polish and Release

Final polish and release preparation.

#### Code Quality

- [ ] Run ruff format on all files
- [ ] Run ruff check and fix all issues
- [ ] Run mypy and fix all type errors
- [ ] Review and update AIDEV-NOTE comments
- [ ] Remove any dead code or unused imports

#### Release Preparation

- [ ] Update version number in `pyproject.toml`
- [ ] Verify all dependencies are correctly specified
- [ ] Test installation from clean environment
- [ ] Test with Python 3.10, 3.11, 3.12, 3.13
- [ ] Create release notes

---

## Usage Examples

### CI/CD Pipeline (GitHub Actions)

```yaml
jobs:
  build:
    steps:
      - name: Report build start
        run: |
          statshed submit -g ci-builds -j "${{ github.repository }}" \
            -s progress -m "Build started: ${{ github.sha }}"

      - name: Build
        run: make build

      - name: Report build success
        if: success()
        run: |
          statshed submit -g ci-builds -j "${{ github.repository }}" \
            -s success -m "Build passed: ${{ github.sha }}"

      - name: Report build failure
        if: failure()
        run: |
          statshed submit -g ci-builds -j "${{ github.repository }}" \
            -s error -m "Build failed: ${{ github.sha }}"
```

### Cron Job with Script Safety

```bash
#!/bin/bash
set -eu

# Submit commands won't cause script to exit on API errors
statshed submit -g backups -j database -s progress -m "Starting backup"

# Do the actual backup
pg_dump mydb > /backups/mydb.sql

# Report success
statshed submit -g backups -j database -s success -m "Backup completed"
```

### Cron Job with Strict Error Handling

```bash
#!/bin/bash
# No set -eu, handle errors manually

if ! statshed submit --strict -g backups -j database -s progress; then
    echo "Warning: Could not report status to dashboard"
fi

pg_dump mydb > /backups/mydb.sql
backup_status=$?

if [ $backup_status -eq 0 ]; then
    statshed submit -g backups -j database -s success
else
    statshed submit -g backups -j database -s error -m "Backup failed with code $backup_status"
fi
```

### Interactive Terminal Session

```bash
# Check overall health with rich output
$ statshed health

# List groups with status summary
$ statshed groups

# Drill into a specific group
$ statshed jobs nightly-builds

# Check group-specific timeout configuration
$ statshed group-config nightly-builds

# Update group timeout (builds can take longer)
$ statshed group-config nightly-builds --progress-timeout 30
```

### Configuration File Example

```yaml
# ~/.config/statshed/statshed.yaml
url: https://statshed.internal.example.com
color: auto
timeout: 30

submit:
  syslog: true
  syslog_facility: local0
  strict: false
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Lenient submit mode by default | Prevents status reporting from breaking scripts with `set -eu` |
| Strict mode for query commands | Users expect errors when querying non-existent resources |
| Optional Rich dependency | Works in minimal environments, enhanced where available |
| YAML config format | Human-readable, widely understood, good library support |
| Click for CLI framework | Already in use, mature, good completion support |
| Syslog support | Standard for daemon/cron job logging, doesn't pollute stdout/stderr |
