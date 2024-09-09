resource "kubernetes_deployment" "trc_app" {
  metadata {
    name      = "backend"
    namespace = "app"
    labels = {
      app = "backend"
    }
    annotations = {
      "prometheus.io/scrape" = "true"
      "prometheus.io/path"   = "/metrics"
      "prometheus.io/port"   = "8080"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        app = "backend"
      }
    }
    template {
      metadata {
        labels = {
          app = "backend"
        }
      }
      spec {
        container {
          name  = "backend"
          image = "us-central1-docker.pkg.dev/trc-challenge/trc-app/trc-app:ea8dd1be5da34c76192fcef0f0feb62b71a96882"
          port {
            container_port = 8080
          }
        
          env {
            name = "PROJECT_ID"
            value_from {
              secret_key_ref {
                name = "backend-secrets"
                key  = "PROJECT_ID"
              }
            }
          }
          env {
            name = "SUBSCRIPTION_ID"
            value_from {
              secret_key_ref {
                name = "backend-secrets"
                key  = "SUBSCRIPTION_ID"
              }
            }
          }
          env {
            name = "TOPIC_NAME"
            value_from {
              secret_key_ref {
                name = "backend-secrets"
                key  = "TOPIC_NAME"
              }
            }
          }
          env {
            name = "SERVICE_ACCOUNT_PATH"
            value = "/app/service-account.json" 
          }
          env {
            name = "OTEL_ENDPOINT"
            value = "otel-collector.monitoring.svc.cluster.local:4317"
          }
          env {
            name = "REDIS_HOST"
            value_from {
              secret_key_ref {
                name = "backend-secrets"
                key  = "REDIS_HOST"
              }
            }
          }
          env {
            name = "BUCKET_NAME"
            value_from {
              secret_key_ref {
                name = "backend-secrets"
                key  = "BUCKET_NAME"
              }
            }
          }

          volume_mount {
            name       = "service-account"
            mount_path = "/app/service-account.json"
            sub_path   = "service-account.json"
            read_only  = true
          }
        }

        volume {
          name = "service-account"
          secret {
            secret_name = "backend-secrets"
            items {
              key  = "SERVICE_ACCOUNT_JSON"
              path = "service-account.json"
            }
          }
        }
      }
    }
  }
}

# Go Application Service
resource "kubernetes_service" "trc_app" {
  metadata {
    name      = "backend"
    namespace = "app"
    annotations = {
      "prometheus.io/scrape" = "true"
      "prometheus.io/port"   = "8080"
      "prometheus.io/path"   = "/metrics"
    }
  }

  spec {
    selector = {
      app = "backend"
    }
    port {
      port        = 8080
      target_port = 8080
    }
    type = "ClusterIP"
  }
}


