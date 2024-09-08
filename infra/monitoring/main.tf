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

