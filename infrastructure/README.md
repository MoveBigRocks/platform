# Move Big Rocks Infrastructure

This directory contains monitoring infrastructure for running Move Big Rocks, the AI-native service operations platform.

## Directory Structure

```
infrastructure/
├── prometheus/
│   └── prometheus.yml           # Prometheus scrape configuration
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── prometheus.yaml  # Prometheus datasource
│   │   └── dashboards/
│   │       └── default.yaml     # Dashboard provisioning config
│   └── dashboards/
│       ├── api-performance.json # API latency and throughput
│       ├── error-monitoring.json # Error rates and types
│       ├── workers.json         # CPU, memory, GC stats
│       ├── database.json        # Database connections, heap
│       ├── storage.json         # Network I/O
│       ├── support-metrics.json # Support case metrics (placeholder)
│       └── tracing.json         # Distributed tracing (placeholder)
└── README.md
```

## Setup

Add monitoring to the server setup by running:

```bash
sudo ./deploy/setup.sh
```

Run that from your private instance repo based on `MoveBigRocks/instance-template`,
not from this core repo.

Then install Prometheus and Grafana:

```bash
# Install Prometheus
apt-get update && apt-get install -y prometheus
cp infrastructure/prometheus/prometheus.yml /etc/prometheus/
echo 'ARGS="--web.listen-address=127.0.0.1:9090"' >> /etc/default/prometheus
systemctl restart prometheus

# Install Grafana
apt-get install -y apt-transport-https software-properties-common
wget -q -O /usr/share/keyrings/grafana.key https://apt.grafana.com/gpg.key
echo "deb [signed-by=/usr/share/keyrings/grafana.key] https://apt.grafana.com stable main" | tee /etc/apt/sources.list.d/grafana.list
apt-get update && apt-get install -y grafana

# Configure Grafana
sed -i 's/^;http_addr =.*/http_addr = 127.0.0.1/' /etc/grafana/grafana.ini

# Copy dashboards
mkdir -p /var/lib/grafana/dashboards
cp infrastructure/grafana/dashboards/*.json /var/lib/grafana/dashboards/
cp infrastructure/grafana/provisioning/datasources/prometheus.yaml /etc/grafana/provisioning/datasources/
cp infrastructure/grafana/provisioning/dashboards/default.yaml /etc/grafana/provisioning/dashboards/
chown -R grafana:grafana /var/lib/grafana/dashboards
systemctl enable grafana-server && systemctl restart grafana-server
```

## Security

- **Prometheus**: Binds to 127.0.0.1:9090 (not publicly accessible)
- **Grafana**: Binds to 127.0.0.1:3000 (not publicly accessible)
- Access via Caddy reverse proxy at https://admin.example.com/grafana/

## Metrics Endpoints

- Move Big Rocks API: http://localhost:8080/metrics
- Prometheus: http://127.0.0.1:9090
- Grafana: http://127.0.0.1:3000
