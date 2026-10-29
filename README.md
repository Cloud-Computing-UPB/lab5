# Laboratorul 5 — Observabilitate și autoscaling în Kubernetes

Repository-ul continuă aplicația `ticketing-api` din laboratoarele anterioare. Codul de
business și PostgreSQL rămân aceleași; în acest laborator adăugăm telemetrie aplicativă,
colectare cu Prometheus, un dashboard Grafana versionat, reguli de alertare și HPA.

Studenții nu trebuie să modifice sursele Go. Fișierele sunt oferite pentru a putea urmări
legătura dintre cod, metrici și resursele Kubernetes.

## Ce este nou

- endpoint Prometheus la `GET /metrics`;
- metrici RED: `http_requests_total`, `http_request_duration_seconds` și
  `http_requests_in_progress`;
- metrica de business `tickets_created_total`;
- loguri structurate cu `request_id`, rută, status și durată;
- `ServiceMonitor`, `PrometheusRule`, dashboard Grafana și HPA în chart-ul Helm;
- generator de trafic și incident controlat prin `ARTIFICIAL_DELAY_MS`, `FAILURE_RATE`
  și `CPU_BURN_MS`;
- configurare reproductibilă pentru Metrics Server și `kube-prometheus-stack`.

## Structură

```text
.
├── charts/ticketing/
│   ├── dashboards/ticketing-overview.json
│   ├── templates/
│   │   ├── servicemonitor.yaml
│   │   ├── prometheusrule.yaml
│   │   ├── grafana-dashboard.yaml
│   │   └── hpa.yaml
│   ├── values-lab5.yaml
│   └── values-incident.yaml
├── internal/observability/
├── monitoring/values.yaml
├── scripts/load.sh
├── scripts/smoke.sh
├── kind-config.yaml
└── Makefile
```

## Cerințe

- Docker
- kubectl
- kind 0.33 sau o versiune compatibilă
- Helm 3
- curl
- jq
- minimum 6 GB RAM disponibili pentru Docker Desktop

Clusterul este fixat la Kubernetes `v1.34.11`, compatibil cu Metrics Server `0.9.x`.
Versiunile chart-urilor folosite în laborator sunt declarate în `Makefile`:

- `metrics-server` chart `3.14.0`;
- `kube-prometheus-stack` chart `92.14.1`.

## Pornire rapidă

```bash
make cluster
make metrics-server
make monitoring
make app
make verify
```

Comenzile echivalente sunt explicate pas cu pas în documentația Docusaurus.

## Imaginea aplicației

Chart-ul folosește implicit imaginea publică:

```text
ghcr.io/cloud-computing-upb/lab5:1.0.0
```

Înainte ca aceasta să fie publicată, imaginea poate fi construită și încărcată direct în
Kind:

```bash
make image
make app
```

## Acces local

În terminale separate:

```bash
kubectl port-forward service/ticketing-api -n cc-lab5 8080:80
kubectl port-forward service/monitoring-kube-prometheus-prometheus -n monitoring 9090:9090
kubectl port-forward service/monitoring-grafana -n monitoring 3000:80
kubectl port-forward service/monitoring-kube-prometheus-alertmanager -n monitoring 9093:9093
```

- API: <http://localhost:8080>
- Prometheus: <http://localhost:9090>
- Grafana: <http://localhost:3000> (`admin` / `admin-lab5`)
- Alertmanager: <http://localhost:9093>

Verificarea metricilor:

```bash
./scripts/smoke.sh
```

## Trafic, incident și recuperare

După pornirea port-forward-ului către API:

```bash
DURATION=120 CONCURRENCY=8 ./scripts/load.sh
```

Incidentul controlat introduce latență, aproximativ 25% răspunsuri `503` și lucru CPU:

```bash
make incident
DURATION=180 CONCURRENCY=12 ./scripts/load.sh
```

Urmărirea HPA și a logurilor:

```bash
kubectl get hpa,pods -n cc-lab5 --watch
kubectl logs deployment/ticketing-api -n cc-lab5 --follow --tail=50
```

Recuperarea readuce valorile normale fără a elimina datele PostgreSQL:

```bash
make recover
```

## SLI și pragurile laboratorului

Dashboard-ul și regulile urmăresc:

- rata cererilor;
- rata răspunsurilor `5xx`, cu prag de alertare de 1%;
- latența p95, cu prag de alertare de 0,5 secunde;
- numărul de replici și utilizarea CPU;
- numărul de tichete create.

Pragurile sunt intenționat stricte pentru ca incidentul să fie vizibil într-un laborator
scurt. Ele nu reprezintă automat obiective potrivite pentru producție.

## Curățare

```bash
make cleanup
```

Ștergerea clusterului Kind elimină și volumele locale ale PostgreSQL.
