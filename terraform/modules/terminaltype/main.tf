data "cloudflare_zone" "dns_zone" {
  name       = var.domain_name_terminal_type
  account_id = "d9c82ad1adf99452890374a6bf5a879a"
}


resource "cloudflare_record" "record" {
  zone_id = data.cloudflare_zone.dns_zone.id
  name    = "@"
  type    = "A"
  value   = var.static_ip
  ttl     = 3600
}


