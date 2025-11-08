# Makefile for building x3270 suite binaries
# This Makefile automates the process of building x3270 binaries for Linux and Windows

# Configuration
X3270_VERSION ?= 4.4ga6
X3270_REPO = https://github.com/pmattes/x3270
X3270_DIR = /tmp/x3270-build
VERSION_FILE = .x3270_version
BINARIES_LINUX = binaries/linux
BINARIES_WINDOWS = binaries/windows

# Binaries to build
LINUX_BINARIES = s3270 x3270if x3270
WINDOWS_BINARIES = s3270.exe x3270if.exe ws3270.exe wc3270.exe

# Colors for output
COLOR_INFO = \033[0;36m
COLOR_SUCCESS = \033[0;32m
COLOR_WARNING = \033[0;33m
COLOR_RESET = \033[0m

.PHONY: all help clean deepclean linux windows check-version test-binaries

# Default target
all: check-version linux windows test-binaries
	@echo "$(COLOR_SUCCESS)✓ All binaries built successfully$(COLOR_RESET)"

# Help target
help:
	@echo "$(COLOR_INFO)x3270 Binary Builder Makefile$(COLOR_RESET)"
	@echo ""
	@echo "Available targets:"
	@echo "  all           - Build all binaries for Linux and Windows (default)"
	@echo "  linux         - Build Linux binaries only"
	@echo "  windows       - Build Windows binaries only"
	@echo "  test-binaries - Test built binaries"
	@echo "  clean         - Remove built binaries (safe, preserves source)"
	@echo "  deepclean     - Remove everything including cloned x3270 source"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Configuration:"
	@echo "  X3270_VERSION - Version to build (default: $(X3270_VERSION))"
	@echo "                  Override with: make X3270_VERSION=4.3ga10 all"
	@echo ""
	@echo "Current version: $(X3270_VERSION)"
	@if [ -f $(VERSION_FILE) ]; then \
		echo "Last built version: $$(cat $(VERSION_FILE))"; \
	else \
		echo "Last built version: none"; \
	fi

# Check if we need to rebuild based on version
check-version:
	@if [ -f $(VERSION_FILE) ] && [ "$$(cat $(VERSION_FILE))" = "$(X3270_VERSION)" ]; then \
		if [ -f $(BINARIES_LINUX)/s3270 ] && [ -f $(BINARIES_WINDOWS)/s3270.exe ]; then \
			echo "$(COLOR_INFO)ℹ Version $(X3270_VERSION) already built, skipping...$(COLOR_RESET)"; \
			echo "$(COLOR_INFO)  Use 'make deepclean' first to force rebuild$(COLOR_RESET)"; \
			exit 0; \
		fi; \
	fi

# Clone or update x3270 repository
$(X3270_DIR):
	@echo "$(COLOR_INFO)→ Cloning x3270 repository...$(COLOR_RESET)"
	@mkdir -p $(X3270_DIR)
	@if [ ! -d $(X3270_DIR)/.git ]; then \
		git clone $(X3270_REPO) $(X3270_DIR); \
	fi
	@cd $(X3270_DIR) && git fetch --all --tags
	@cd $(X3270_DIR) && git checkout $(X3270_VERSION)
	@echo "$(COLOR_SUCCESS)✓ Repository ready at version $(X3270_VERSION)$(COLOR_RESET)"

# Build Linux binaries
linux: $(X3270_DIR)
	@echo "$(COLOR_INFO)→ Building Linux binaries...$(COLOR_RESET)"
	@mkdir -p $(BINARIES_LINUX)
	
	# Configure and build for Linux
	@cd $(X3270_DIR) && \
		./configure --prefix=/usr/local --enable-s3270 --enable-x3270if 2>&1 | tail -20 && \
		make clean && \
		make -j$$(nproc) 2>&1 | tail -20
	
	# Copy Linux binaries
	@for binary in $(LINUX_BINARIES); do \
		if [ -f $(X3270_DIR)/$$binary/$$binary ]; then \
			echo "  Copying $$binary..."; \
			cp $(X3270_DIR)/$$binary/$$binary $(BINARIES_LINUX)/$$binary; \
			strip $(BINARIES_LINUX)/$$binary 2>/dev/null || true; \
			chmod +x $(BINARIES_LINUX)/$$binary; \
		elif [ -f $(X3270_DIR)/s3270/$$binary ]; then \
			echo "  Copying $$binary from s3270..."; \
			cp $(X3270_DIR)/s3270/$$binary $(BINARIES_LINUX)/$$binary; \
			strip $(BINARIES_LINUX)/$$binary 2>/dev/null || true; \
			chmod +x $(BINARIES_LINUX)/$$binary; \
		else \
			echo "$(COLOR_WARNING)⚠ Warning: $$binary not found$(COLOR_RESET)"; \
		fi \
	done
	
	@echo "$(COLOR_SUCCESS)✓ Linux binaries built$(COLOR_RESET)"

# Build Windows binaries
windows: $(X3270_DIR)
	@echo "$(COLOR_INFO)→ Building Windows binaries...$(COLOR_RESET)"
	@mkdir -p $(BINARIES_WINDOWS)
	
	# Check for MinGW cross-compiler
	@if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		echo "$(COLOR_WARNING)⚠ MinGW cross-compiler not found, installing...$(COLOR_RESET)"; \
		sudo apt-get update -qq && sudo apt-get install -y -qq mingw-w64 2>&1 | tail -10; \
	fi
	
	# Configure and build for Windows
	@cd $(X3270_DIR) && \
		./configure --host=x86_64-w64-mingw32 --prefix=/usr/local --enable-s3270 --enable-x3270if 2>&1 | tail -20 && \
		make clean && \
		make -j$$(nproc) 2>&1 | tail -20
	
	# Copy Windows binaries
	@for binary in $(WINDOWS_BINARIES); do \
		base=$$(echo $$binary | sed 's/.exe$$//'); \
		if [ -f $(X3270_DIR)/$$base/$$binary ]; then \
			echo "  Copying $$binary..."; \
			cp $(X3270_DIR)/$$base/$$binary $(BINARIES_WINDOWS)/$$binary; \
			x86_64-w64-mingw32-strip $(BINARIES_WINDOWS)/$$binary 2>/dev/null || true; \
		elif [ -f $(X3270_DIR)/s3270/$$binary ]; then \
			echo "  Copying $$binary from s3270..."; \
			cp $(X3270_DIR)/s3270/$$binary $(BINARIES_WINDOWS)/$$binary; \
			x86_64-w64-mingw32-strip $(BINARIES_WINDOWS)/$$binary 2>/dev/null || true; \
		else \
			echo "$(COLOR_WARNING)⚠ Warning: $$binary not found$(COLOR_RESET)"; \
		fi \
	done
	
	@echo "$(COLOR_SUCCESS)✓ Windows binaries built$(COLOR_RESET)"

# Test binaries after build
test-binaries:
	@echo "$(COLOR_INFO)→ Testing built binaries...$(COLOR_RESET)"
	@echo ""
	@echo "Linux binaries:"
	@for binary in $(LINUX_BINARIES); do \
		if [ -f $(BINARIES_LINUX)/$$binary ]; then \
			size=$$(du -h $(BINARIES_LINUX)/$$binary | cut -f1); \
			echo "  ✓ $$binary ($$size)"; \
		else \
			echo "  ✗ $$binary (missing)"; \
			exit 1; \
		fi \
	done
	@echo ""
	@echo "Windows binaries:"
	@for binary in $(WINDOWS_BINARIES); do \
		if [ -f $(BINARIES_WINDOWS)/$$binary ]; then \
			size=$$(du -h $(BINARIES_WINDOWS)/$$binary | cut -f1); \
			echo "  ✓ $$binary ($$size)"; \
		else \
			echo "  ✗ $$binary (missing)"; \
			exit 1; \
		fi \
	done
	@echo ""
	
	# Test Linux s3270 binary
	@if [ -f $(BINARIES_LINUX)/s3270 ]; then \
		echo "Testing s3270 (Linux):"; \
		$(BINARIES_LINUX)/s3270 -v 2>&1 | head -3 || true; \
		echo ""; \
	fi
	
	# Save version
	@echo "$(X3270_VERSION)" > $(VERSION_FILE)
	@echo "$(COLOR_SUCCESS)✓ All binaries present and accounted for$(COLOR_RESET)"
	@echo "$(COLOR_SUCCESS)✓ Version $(X3270_VERSION) saved to $(VERSION_FILE)$(COLOR_RESET)"

# Clean built binaries (safe - keeps source)
clean:
	@echo "$(COLOR_INFO)→ Cleaning built binaries...$(COLOR_RESET)"
	@rm -f $(BINARIES_LINUX)/*
	@rm -f $(BINARIES_WINDOWS)/*
	@rm -f $(VERSION_FILE)
	@echo "$(COLOR_SUCCESS)✓ Binaries cleaned (source preserved in $(X3270_DIR))$(COLOR_RESET)"

# Deep clean - removes everything including source
deepclean: clean
	@echo "$(COLOR_INFO)→ Performing deep clean...$(COLOR_RESET)"
	@rm -rf $(X3270_DIR)
	@echo "$(COLOR_SUCCESS)✓ Deep clean complete (all build artifacts removed)$(COLOR_RESET)"
