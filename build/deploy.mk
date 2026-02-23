# Deploy

# Deploy single gateway for local demo (mcp-gateway in gateway-system)
# For e2e tests with multiple gateways, use deploy-e2e-gateways instead
.PHONY: deploy-gateway
deploy-gateway: $(KUSTOMIZE) ## Deploy single MCP gateway for local demo
	$(KUSTOMIZE) build config/istio/gateway | kubectl apply -f -

.PHONY: undeploy-gateway
undeploy-gateway: $(KUSTOMIZE) ## Remove the MCP gateway
	- $(KUSTOMIZE) build config/istio/gateway | kubectl delete -f -

.PHONY: deploy-namespaces
deploy-namespaces: # Create MCP namespaces
	kubectl apply -f config/mcp-system/namespace.yaml
	kubectl apply -f config/istio/gateway/namespace.yaml	
