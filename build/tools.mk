# Tools

KIND = bin/kind
KIND_VERSION = v0.29.0
$(KIND):
	GOBIN=$(PWD)/bin go install sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: kind
kind: $(KIND) # Download kind locally if necessary.

HELM = bin/helm
HELM_VERSION = v3.18.6
$(HELM):
	mkdir -p bin
	curl -fsSL https://get.helm.sh/helm-$(HELM_VERSION)-$(OS)-$(ARCH).tar.gz | tar -xz -C bin --strip-components=1 $(OS)-$(ARCH)/helm

.PHONY: helm
helm: $(HELM) # Download helm locally if necessary.

YQ = bin/yq
YQ_VERSION = v4.47.1
$(YQ):
	GOBIN=$(PWD)/bin go install github.com/mikefarah/yq/v4@$(YQ_VERSION)

.PHONY: yq
yq: $(YQ) # Download yq locally if necessary.

KUSTOMIZE = bin/kustomize
KUSTOMIZE_VERSION = v5.7.1
$(KUSTOMIZE):
	GOBIN=$(PWD)/bin go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) # Download kustomize locally if necessary.

GOLANGCI_LINT = bin/golangci-lint
GOLANGCI_LINT_VERSION = v2.4.0
$(GOLANGCI_LINT):
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: golangci-lint-bin
golangci-lint-bin: $(GOLANGCI_LINT) # Download golangci-lint locally if necessary.
