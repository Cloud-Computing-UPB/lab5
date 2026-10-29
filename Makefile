CLUSTER_NAME ?= cc-lab5
NAMESPACE ?= cc-lab5
MONITORING_NAMESPACE ?= monitoring
IMAGE ?= ghcr.io/cloud-computing-upb/lab5:1.0.0
METRICS_SERVER_CHART_VERSION ?= 3.14.0
KUBE_PROMETHEUS_STACK_VERSION ?= 92.14.1

.PHONY: cluster image metrics-server monitoring app verify status load incident recover cleanup

cluster:
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml

image:
	docker build -t $(IMAGE) .
	kind load docker-image $(IMAGE) --name $(CLUSTER_NAME)

metrics-server:
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ --force-update
	helm upgrade --install metrics-server metrics-server/metrics-server \
		--namespace kube-system \
		--version $(METRICS_SERVER_CHART_VERSION) \
		--set 'args={--kubelet-insecure-tls,--kubelet-preferred-address-types=InternalIP}' \
		--wait --timeout 3m

monitoring:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
		--namespace $(MONITORING_NAMESPACE) --create-namespace \
		--version $(KUBE_PROMETHEUS_STACK_VERSION) \
		--values monitoring/values.yaml \
		--wait --timeout 10m

app:
	helm upgrade --install ticketing charts/ticketing \
		--namespace $(NAMESPACE) --create-namespace \
		--values charts/ticketing/values-lab5.yaml \
		--wait --timeout 5m

verify:
	helm test ticketing --namespace $(NAMESPACE) --logs
	kubectl get servicemonitor,prometheusrule,hpa --namespace $(NAMESPACE)

status:
	kubectl get pods --namespace $(NAMESPACE) -o wide
	kubectl top pods --namespace $(NAMESPACE)
	kubectl get hpa --namespace $(NAMESPACE)

load:
	./scripts/load.sh

incident:
	helm upgrade ticketing charts/ticketing \
		--namespace $(NAMESPACE) \
		--values charts/ticketing/values-lab5.yaml \
		--values charts/ticketing/values-incident.yaml \
		--wait --timeout 5m

recover:
	helm upgrade ticketing charts/ticketing \
		--namespace $(NAMESPACE) \
		--values charts/ticketing/values-lab5.yaml \
		--wait --timeout 5m

cleanup:
	-helm uninstall ticketing --namespace $(NAMESPACE)
	-kind delete cluster --name $(CLUSTER_NAME)
