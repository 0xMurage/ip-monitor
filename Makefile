BUILD_OUTPUT_FILENAME ?= monitor

# Allow user to specify GOOS and GOARCH
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ifeq ($(GOOS),windows)
	ifneq ($(suffix $(BUILD_OUTPUT_FILENAME)),.exe)
		BUILD_OUTPUT_FILENAME := $(addsuffix .exe,$(BUILD_OUTPUT_FILENAME))
	endif
endif

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o dist/$(BUILD_OUTPUT_FILENAME) $(BUILD_FLAGS) .