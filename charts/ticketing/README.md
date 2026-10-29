# Chart-ul `ticketing`

Chart-ul instalează Ticketing API și PostgreSQL. Resursele de observabilitate sunt
condiționale, astfel încât chart-ul rămâne instalabil și fără Prometheus Operator.

Validare de bază:

```bash
helm lint charts/ticketing --strict
helm template ticketing charts/ticketing --namespace cc-lab5
```

După instalarea `kube-prometheus-stack`, overlay-ul laboratorului activează
`ServiceMonitor`, `PrometheusRule`, dashboard-ul Grafana și HPA:

```bash
helm upgrade --install ticketing charts/ticketing \
  --namespace cc-lab5 \
  --create-namespace \
  --values charts/ticketing/values-lab5.yaml \
  --wait --timeout 5m
```

Incidentul controlat se aplică peste configurația de laborator:

```bash
helm upgrade ticketing charts/ticketing \
  --namespace cc-lab5 \
  --values charts/ticketing/values-lab5.yaml \
  --values charts/ticketing/values-incident.yaml \
  --wait --timeout 5m
```

Ordinea fișierelor `--values` este importantă: ultimul fișier suprascrie numai valorile
pe care le declară.
