# Helm install test

.PHONY: test-helm-install
test-helm-install: kind helm kustomize yq ## Run helm install test against a clean Kind cluster
	bash tests/helm/test-helm-install.sh
