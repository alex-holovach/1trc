resource "kubernetes_config_map" "grafana_dashboard" {
  metadata {
    name      = "grafana-dashboard"
    namespace = "monitoring"
    labels = {
      grafana_dashboard = "Services"
    }
  }

  data = {
    "dashboard.json" = file("${path.module}/dashboards/services.json")
  }
}

resource "helm_release" "grafana" {
  name       = "grafana"
  repository = "https://grafana.github.io/helm-charts"
  chart      = "grafana"
  version    = "6.50.7"
  namespace  = "monitoring"

  values = [
    file("${path.module}/grafana-values.yaml")
  ]

  set {
    name  = "dashboards.default.my-dashboard.file"
    value = "/tmp/dashboards/dashboard.json"
  }

  depends_on = [kubernetes_config_map.grafana_dashboard]
}

resource "helm_release" "prometheus" {
  name       = "prometheus"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus"
  namespace  = "monitoring"

  values = [
    file("${path.module}/prometheus-values.yaml")
  ]
}

resource "kubernetes_config_map" "otel_collector_config" {
  metadata {
    name      = "otel-collector-config"
    namespace = "monitoring"
  }

  data = {
    "config.yaml" = <<-EOT
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317

    processors:
      batch:

    exporters:
      otlp:
        endpoint: "tempo.monitoring.svc.cluster.local:9096"
        tls:
          insecure: true
    service:
      telemetry:
        logs:
          level: "debug"
      pipelines:
        traces:
          receivers: [otlp]
          processors: []
          exporters: [otlp]
    EOT
  }
}

# OpenTelemetry Collector Deployment
resource "kubernetes_deployment" "otel_collector" {
  metadata {
    name      = "otel-collector"
    namespace = "monitoring"
    labels = {
      app = "otel-collector"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "otel-collector"
      }
    }

    template {
      metadata {
        labels = {
          app = "otel-collector"
        }
      }

      spec {
        container {
          image = "otel/opentelemetry-collector:latest"
          name  = "otel-collector"

          args = ["--config", "/conf/config.yaml"]

          port {
            container_port = 4317
            name           = "otlp-grpc"
          }

          volume_mount {
            name       = "config"
            mount_path = "/conf"
          }

          resources {
            limits = {
              cpu    = "500m"
              memory = "512Mi"
            }
            requests = {
              cpu    = "200m"
              memory = "256Mi"
            }
          }
        }

        volume {
          name = "config"
          config_map {
            name = kubernetes_config_map.otel_collector_config.metadata[0].name
          }
        }
      }
    }
  }
}

# OpenTelemetry Collector Service
resource "kubernetes_service" "otel_collector" {
  metadata {
    name      = "otel-collector"
    namespace = "monitoring"
  }
  spec {
    selector = {
      app = "otel-collector"
    }
    port {
      port        = 4317
      target_port = 4317
      name        = "otlp-grpc"
    }
    type = "ClusterIP"
  }
}

# Tempo ConfigMap
resource "kubernetes_config_map" "tempo_config" {
  metadata {
    name      = "tempo-config"
    namespace = "monitoring"
  }

  data = {
    "tempo.yaml" = <<-EOT
    server:
      http_listen_port: 3100

    distributor:
      receivers:
        otlp:
          protocols:
            grpc:
              endpoint: "0.0.0.0:9096"

    ingester:
      trace_idle_period: 10s
      max_block_duration: 5m

    compactor:
      compaction:
        block_retention: 1h

    storage:
      trace:
        backend: local
        local:
          path: /tmp/tempo/blocks
    EOT
  }
}

resource "kubernetes_deployment" "tempo" {
  metadata {
    name      = "tempo"
    namespace = "monitoring"
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "tempo"
      }
    }

    template {
      metadata {
        labels = {
          app = "tempo"
        }
      }

      spec {
        container {
          image = "grafana/tempo:latest"
          name  = "tempo"

          port {
            container_port = 3100
            name           = "http"
          }

          port {
            container_port = 9096
            name           = "grpc"
          }

          args = ["-config.file=/etc/tempo.yaml"]

          volume_mount {
            name       = "tempo-config"
            mount_path = "/etc/tempo.yaml"
            sub_path   = "tempo.yaml"
          }
        }

        volume {
          name = "tempo-config"
          config_map {
            name = kubernetes_config_map.tempo_config.metadata[0].name
          }
        }
      }
    }
  }
}

# Tempo Service
resource "kubernetes_service" "tempo" {
  metadata {
    name      = "tempo"
    namespace = "monitoring"
  }

  spec {
    selector = {
      app = "tempo"
    }

    port {
      port        = 3100
      target_port = 3100
      name        = "http"
    }

    port {
      port        = 9096
      target_port = 9096
      name        = "grpc"
    }
  }
}


