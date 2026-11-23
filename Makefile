BIN_DIR ?= $(HOME)/.local/bin
BIN     := timesheet

.PHONY: build install uninstall test

build:
	go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/timesheet

install: build
	mkdir -p $(BIN_DIR)
	install -m 755 $(BIN) $(BIN_DIR)/$(BIN)
	@echo "Installed to $(BIN_DIR)/$(BIN)"
	@case ":$$PATH:" in *":$(BIN_DIR):"*) ;; *) echo "NOTE: add $(BIN_DIR) to your PATH";; esac

uninstall:
	rm -f $(BIN_DIR)/$(BIN)

test:
	go test ./...
