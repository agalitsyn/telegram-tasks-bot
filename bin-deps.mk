GOLANGCI_BIN ?= $(GOBIN)/golangci-lint
GOLANGCI_VERSION ?= v1.62.2

.PHONY: $(GOLANGCI_BIN)
$(GOLANGCI_BIN):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
