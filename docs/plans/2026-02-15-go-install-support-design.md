# Design: Standard Go Package with go install Support

**Date:** 2026-02-15
**Status:** Approved

## Overview

Transform parta into a standard Go package that can be installed via `go install`, and rename the thread parameter from `-p` to `-t` for better clarity.

## Goals

1. Support standard `go install` installation method
2. Rename `-p` parameter to `-t` for thread/concurrent tasks
3. Update README documentation to reflect these changes
4. Maintain backward compatibility with existing functionality

## Approach

We chose **Approach 1: Minimal Restructuring** to achieve the core requirements with minimal risk and changes.

## Directory Structure Changes

### Current Structure
```
parta/
├── cmd/app/main.go
├── pkg/gpool/gpool.go
├── go.mod
├── go.sum
├── .goreleaser.yml
└── README.md
```

### New Structure
```
parta/
├── cmd/parta/main.go  (moved from cmd/app/main.go)
├── pkg/gpool/gpool.go (unchanged)
├── go.mod            (unchanged)
├── go.sum            (unchanged)
├── .goreleaser.yml   (updated path)
└── README.md         (updated)
```

### Rationale

- `cmd/parta/main.go` follows Go conventions for multi-binary projects
- Binary name will be `parta` when installed via `go install`
- Standard Go users will find the structure familiar
- Installation command: `go install github.com/seqyuan/parta/cmd/parta@latest`

## Parameter Changes

### From `-p` to `-t`

**Before:**
```bash
./parta -i input.sh -l 2 -p 2
```

**After:**
```bash
parta -i input.sh -l 2 -t 2
```

### Code Changes

In `cmd/parta/main.go`:
- Line 316: `opt_p` → `opt_t`, keep short flag as "t"
- Line 328: `*opt_p` → `*opt_t`
- Help text already says "thread", so flag now matches description

### Rationale

- `-t` better represents "thread" than `-p`
- Shorter and more intuitive
- No backward compatibility needed per user preference

## README Documentation Updates

### New Installation Section

Add before "程序功能" section:

```markdown
# Installation

## Using go install (Recommended)
```bash
go install github.com/seqyuan/parta/cmd/parta@latest
```

This installs `parta` to your `$GOPATH/bin` or `$GOBIN` directory.

## From source
```bash
git clone https://github.com/seqyuan/parta.git
cd parta
go build -o parta ./cmd/parta
```
```

### Parameter Documentation Updates

Update section "程序参数":
- Change `-p  --thread` to `-t  --thread`

### Usage Example Updates

Replace all occurrences of `-p` with `-t`:
- Line 34: `./parta -i input.sh -l 2 -p 2` → `parta -i input.sh -l 2 -t 2`
- Line 92: Same change
- Line 103: Same change

Also change `./parta` to `parta` to reflect installed binary usage.

### Cleanup

- Remove line 160 (manual git tag command) - internal maintenance info

## .goreleaser.yml Updates

Update the main path:
```yaml
main: ./cmd/parta/main.go  # was ./cmd/app/main.go
```

## Implementation Checklist

1. Move `cmd/app/` directory to `cmd/parta/`
2. Update `.goreleaser.yml` main path
3. Update all parameter references in main.go (opt_p → opt_t)
4. Update README.md:
   - Add installation section
   - Update parameter documentation
   - Update all usage examples
   - Remove internal maintenance line
5. Test the changes:
   - Build locally: `go build ./cmd/parta`
   - Verify parameters work: `./parta -h`
   - Test installation (optional): `go install ./cmd/parta`

## Non-Goals

- Code refactoring or reorganization
- Adding new features
- Changing existing functionality
- Breaking changes to command behavior

## Risks and Mitigations

**Risk:** Users with scripts using `-p` will break
**Mitigation:** Accepted per user preference - this is a breaking change that simplifies the interface

**Risk:** goreleaser CI might fail initially
**Mitigation:** The path update in `.goreleaser.yml` should handle this

## Success Criteria

- [ ] `go install github.com/seqyuan/parta/cmd/parta@latest` works
- [ ] Binary is named `parta` after installation
- [ ] `-t` parameter works for thread count
- [ ] `-p` parameter no longer exists
- [ ] README accurately describes installation and usage
- [ ] goreleaser builds successfully
