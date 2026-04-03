# DNS Management

Jabali uses PowerDNS with a REST API and MySQL backend for DNS zone management.

## Features

- REST API integration (panel calls PowerDNS directly, no agent involvement)
- MySQL backend for zone storage
- DNSSEC via PowerDNS cryptokeys API
- Zone templates for quick setup
- All standard record types (A, AAAA, CNAME, MX, TXT, SRV, NS, CAA, etc.)

## Auto-Configuration

When a domain is created, Jabali automatically sets up:

- SOA record
- NS records (pointing to configured nameservers)
- A record (server IP)
- MX record (mail.$domain)
- SPF, DKIM, DMARC TXT records for email
- Autoconfig/autodiscover records for mail client setup

## DNSSEC

Enable/disable DNSSEC per domain from the DNS Zones page. Jabali manages:

- Key generation via PowerDNS cryptokeys API
- DS record display for registrar configuration
- Key rotation

## CLI Commands

```bash
jabali dns list                          # List all DNS zones
jabali dns records <domain>              # List records for a domain
jabali dns records <domain> --type=MX    # Filter by record type
jabali dns add <domain> <name> <type> <content> [--ttl=3600] [--priority=0]
jabali dns delete-record <domain> --type=A --name=www --value=1.2.3.4
jabali dns sync <domain>                 # Sync zone with PowerDNS
```
