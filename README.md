# timesheet-cli

Fill in your monthly timesheet (.xlsx) from the terminal. Go, standard library only.

The workbook is the source of truth. Each tab is expected to have the header
`No. | Date | Check In | Check Out | Work Place | Task List` and one pre-filled row per working day.
The CLI finds the row by date (the right tab is picked automatically) and fills in only that row.
Every other part of the file (images, styles, other tabs) is copied byte-for-byte.

## Install

Requires [Go](https://go.dev/dl/) 1.22 or newer.

    git clone <repo-url> timesheet-cli && cd timesheet-cli
    make install                       # builds and copies to ~/.local/bin/timesheet
    make install BIN_DIR=/usr/local/bin   # other location (may need sudo)

Make sure `~/.local/bin` is in your `PATH` (zsh: add `export PATH="$HOME/.local/bin:$PATH"` to `~/.zshrc`), then check:

    timesheet help

Without make: `go build -trimpath -ldflags="-s -w" -o ~/.local/bin/timesheet ./cmd/timesheet`

Update: run `make install` again. Uninstall: `make uninstall` (config and backups stay in `~/.timesheet/`).

## Usage

    timesheet sheet link ~/Documents/iglo/timesheet/oktober.xlsx   # once; name defaults to "oktober"
    timesheet                       # fill in today: asks check in, check out, work place, task
    timesheet add --in now          # arrived: just stamp the check-in
    timesheet add --out 16:45 "fix login"   # leaving: check out + task
    timesheet add --date 2026-11-19 --in 7.30 --out 16:30 --task "sprint planning"
    timesheet list                  # this month; days still empty show "-"
    timesheet list --month 2026-10
    timesheet rm 2026-11-19         # clear a day (asks first)
    timesheet export --month 2026-11 --to nov.csv
    timesheet sheet                 # show linked files, pick one
    timesheet sheet november        # switch the active file

Times accept `7:30`, `07:30`, `7.30` and `now`. In the prompts, Enter keeps the current value and `-` clears it.

## Safety

- A copy of the workbook is saved to `~/.timesheet/backups/` before the first write of each day.
- If Excel's lock file (`~$name.xlsx`) exists, the CLI asks before writing (`--force` skips the question).
  A stale lock file left by a crash can be deleted when Excel is closed.
- After writing, the row is read back; a mismatch is reported instead of being ignored.
- Cells that contain formulas are never overwritten.
- Weekends and dates without a row are rejected: the CLI never inserts rows or moves the signature block.

Config is `~/.timesheet/config.json` (override the folder with `TIMESHEET_HOME`).

## Shell completion

    source <(timesheet completion bash)                  # add to ~/.bashrc
    timesheet completion zsh > "${fpath[1]}/_timesheet"   # then restart zsh
    timesheet completion fish > ~/.config/fish/completions/timesheet.fish

## Layout

- `cmd/timesheet` entry point
- `internal/cli` argument parsing, prompts, output
- `internal/timesheet` entry model, service (rules, time parsing), `Repository` interface, config, report
- `internal/xlsx` the `Repository` for .xlsx files (zip, sheet XML, backups, lock check); `xlsxtest` builds fixtures for tests

Tests: `go test ./...`

## License

[MIT](LICENSE) © 2026 ilhamnrachman
