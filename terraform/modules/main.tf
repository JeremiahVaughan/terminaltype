data "cloudflare_zone" "dns_zone" {
  name       =  "terminaltype.com"
#  name       = var.domain_name_terminal_type
  account_id = "d9c82ad1adf99452890374a6bf5a879a"
}


resource "cloudflare_record" "record" {
  zone_id = data.cloudflare_zone.dns_zone.id
  name    = "@"
  type    = "A"
  value   = var.ip_address
  ttl     = 3600
}


