BINARY_NAME = mcpm
GO = go
GOPATH ?= $(shell go env GOPATH)
INSTALL_DIR = $(GOPATH)/bin
TEST_FLAGS = -race -cover

.PHONY: build install clean test coverage run

build:
	$(GO) build -o $(BINARY_NAME) .

install: build
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(BINARY_NAME) installed to $(INSTALL_DIR)/$(BINARY_NAME)"

clean:
	@rm -f $(BINARY_NAME)
	@echo "Cleaned $(BINARY_NAME)"

test:
	$(GO) test $(TEST_FLAGS) -v -coverprofile=coverage.out ./...
	@echo ""
	@$(GO) tool cover -func=coverage.out | grep "total:"

coverage: test
	@$(GO) tool cover -func=coverage.out | grep "total:"

run: build
	./$(BINARY_NAME)
