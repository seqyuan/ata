# Go Install Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform parta into a standard Go package installable via `go install` and rename the thread parameter from `-p` to `-t`.

**Architecture:** Restructure from `cmd/app` to `cmd/parta` following Go conventions for installable binaries. Update all references to the renamed parameter throughout code and documentation.

**Tech Stack:** Go 1.22.2, goreleaser for releases, sqlite3 for job tracking

---

## Task 1: Restructure Directory for go install Support

**Files:**
- Move: `cmd/app/` → `cmd/parta/`
- Modify: `cmd/parta/main.go` (after move)

**Step 1: Create new directory structure**

Run:
```bash
mkdir -p cmd/parta
```

Expected: Directory created successfully

**Step 2: Move main.go to new location**

Run:
```bash
mv cmd/app/main.go cmd/parta/main.go
```

Expected: File moved successfully

**Step 3: Remove old directory**

Run:
```bash
rmdir cmd/app
```

Expected: Directory removed (should be empty after move)

**Step 4: Verify the move**

Run:
```bash
ls -la cmd/parta/main.go
```

Expected: File exists at new location

**Step 5: Test build with new path**

Run:
```bash
go build ./cmd/parta
```

Expected: Binary `parta` created in current directory, no errors

**Step 6: Commit directory restructure**

```bash
git add -A
git commit -m "refactor: move cmd/app to cmd/parta for go install support"
```

---

## Task 2: Rename Parameter from -p to -t

**Files:**
- Modify: `cmd/parta/main.go:316` (parser definition)
- Modify: `cmd/parta/main.go:328` (usage)

**Step 1: Update parser definition**

In `cmd/parta/main.go` line 316, change:
```go
opt_p := parser.Int("p", "thread", &argparse.Options{Default: 1, Help: "Max concurrent tasks to run (default: 1)"})
```

To:
```go
opt_t := parser.Int("t", "thread", &argparse.Options{Default: 1, Help: "Max concurrent tasks to run (default: 1)"})
```

**Step 2: Update parameter usage**

In `cmd/parta/main.go` line 328, change:
```go
IlterCommand(dbObj, *opt_p, need2run)
```

To:
```go
IlterCommand(dbObj, *opt_t, need2run)
```

**Step 3: Test parameter help**

Run:
```bash
go run ./cmd/parta -h
```

Expected output should show:
```
  -t --thread    Max concurrent tasks to run (default: 1)
```

And should NOT show `-p`

**Step 4: Test parameter functionality**

Create a test input file:
```bash
echo -e "echo test1\necho test2\necho test3" > /tmp/test_input.sh
```

Run:
```bash
go run ./cmd/parta -i /tmp/test_input.sh -l 1 -t 2
```

Expected: Program runs successfully, shows task execution

**Step 5: Verify old parameter fails**

Run:
```bash
go run ./cmd/parta -i /tmp/test_input.sh -l 1 -p 2
```

Expected: Error message about unknown flag `-p`

**Step 6: Clean up test files**

Run:
```bash
rm -rf /tmp/test_input.sh /tmp/test_input.sh.db /tmp/test_input.sh.shell
```

**Step 7: Commit parameter change**

```bash
git add cmd/parta/main.go
git commit -m "refactor: rename -p parameter to -t for thread count"
```

---

## Task 3: Update goreleaser Configuration

**Files:**
- Modify: `.goreleaser.yml:18`

**Step 1: Update main path in goreleaser**

In `.goreleaser.yml` line 18, change:
```yaml
    main: ./cmd/app/main.go
```

To:
```yaml
    main: ./cmd/parta/main.go
```

**Step 2: Verify goreleaser config syntax**

Run:
```bash
# Only if goreleaser is installed
goreleaser check 2>/dev/null || echo "goreleaser not installed, skipping validation"
```

Expected: Either "config is valid" or skip message

**Step 3: Commit goreleaser update**

```bash
git add .goreleaser.yml
git commit -m "chore: update goreleaser to use cmd/parta path"
```

---

## Task 4: Update README Documentation

**Files:**
- Modify: `README.md`

**Step 1: Add Installation section**

After line 8 (after the `---`), add new Installation section:

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

**Step 2: Update parameter documentation**

On line 29, change:
```markdown
-p  --thred   Thread process at same time. Default: 1
```

To:
```markdown
-t  --thread   Thread process at same time. Default: 1
```

(Also fixes the typo "thred" → "thread")

**Step 3: Update usage examples**

Replace all occurrences:

Line 34:
```markdown
`./parta -i input.sh -l 2 -p 2`
```
→
```markdown
`parta -i input.sh -l 2 -t 2`
```

Line 103:
```markdown
`./parta -i input.sh -l 2 -p 2`
```
→
```markdown
`parta -i input.sh -l 2 -t 2`
```

**Step 4: Remove maintenance note**

Delete line 160:
```markdown
# update
export version="v1.4.0" && git add -A  && git commit -m $version && git push && git tag $version && git push origin $version
```

**Step 5: Verify README formatting**

Run:
```bash
head -50 README.md
```

Expected: Installation section appears after line 8, formatted correctly

**Step 6: Commit README updates**

```bash
git add README.md
git commit -m "docs: update README with go install instructions and -t parameter"
```

---

## Task 5: Final Verification and Testing

**Files:**
- Test: `cmd/parta/main.go`
- Verify: Build and install process

**Step 1: Clean build**

Run:
```bash
rm -f parta
go clean
```

Expected: No build artifacts remain

**Step 2: Build from new path**

Run:
```bash
go build -o parta ./cmd/parta
```

Expected: Binary `parta` created successfully

**Step 3: Test help output**

Run:
```bash
./parta -h
```

Expected output includes:
- Program name: "parta"
- `-i --infile` parameter
- `-l --line` parameter
- `-t --thread` parameter (NOT `-p`)

**Step 4: Test with sample input**

Create test file:
```bash
cat > /tmp/sample_work.sh <<'EOF'
echo "Task 1"
sleep 1
echo "Task 2"
sleep 1
echo "Task 3"
EOF
```

Run:
```bash
./parta -i /tmp/sample_work.sh -l 1 -t 2
```

Expected:
- Creates `/tmp/sample_work.sh.db`
- Creates `/tmp/sample_work.sh.shell/` directory
- Executes tasks successfully
- Shows output: "All works: 3", "Successed: 3", "Error: 0"

**Step 5: Verify database creation**

Run:
```bash
sqlite3 /tmp/sample_work.sh.db "SELECT COUNT(*) FROM job;"
```

Expected: Returns `3`

**Step 6: Clean up test artifacts**

Run:
```bash
rm -rf /tmp/sample_work.sh /tmp/sample_work.sh.db /tmp/sample_work.sh.shell
```

**Step 7: Test local install (optional)**

Run:
```bash
go install ./cmd/parta
which parta
```

Expected: Shows path to installed binary in `$GOPATH/bin` or `$GOBIN`

**Step 8: Verify all tests pass**

Run:
```bash
go test ./...
```

Expected: All tests pass (or "no test files" if none exist)

**Step 9: Check git status**

Run:
```bash
git status
```

Expected: Clean working tree, all changes committed

---

## Task 6: Create Summary Commit (Optional)

**Files:**
- Verify: All previous commits

**Step 1: Review commit history**

Run:
```bash
git log --oneline -5
```

Expected: Shows all commits from this implementation:
1. docs: update README with go install instructions and -t parameter
2. chore: update goreleaser to use cmd/parta path
3. refactor: rename -p parameter to -t for thread count
4. refactor: move cmd/app to cmd/parta for go install support

**Step 2: Verify changes work end-to-end**

Run:
```bash
go build ./cmd/parta && ./parta -h
```

Expected: Build succeeds, help shows correct parameters

**Step 3: Document completion**

Review checklist from design doc:
- [x] `go install github.com/seqyuan/parta/cmd/parta@latest` works (structure ready)
- [x] Binary is named `parta` after installation
- [x] `-t` parameter works for thread count
- [x] `-p` parameter no longer exists
- [x] README accurately describes installation and usage
- [x] goreleaser builds successfully (config updated)

---

## Success Criteria

1. ✅ Directory structure follows Go conventions (`cmd/parta/main.go`)
2. ✅ Parameter renamed from `-p` to `-t` throughout codebase
3. ✅ README includes installation instructions using `go install`
4. ✅ All usage examples updated to use `-t` and `parta` command
5. ✅ goreleaser configuration points to new path
6. ✅ Binary builds successfully: `go build ./cmd/parta`
7. ✅ Help text shows `-t --thread` parameter
8. ✅ Program executes tasks correctly with new parameter

## Notes

- This is a breaking change: users with scripts using `-p` must update to `-t`
- After merging, tag a new version (e.g., v1.5.0) to indicate breaking change
- Users can install with: `go install github.com/seqyuan/parta/cmd/parta@v1.5.0`
- goreleaser will need the updated path for future releases
