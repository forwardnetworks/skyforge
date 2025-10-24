# Skyforge Demo Workflow (1.0 Release)

> Use `scripts/generate_demo_doc.py` after a successful apply to refresh this playbook with live object names. The version below documents the expected flows and artefacts for the stock demo.tfvars files.

## 1. Bootstrap & Planning

```bash
source .env.local
./bin/terraform init
./bin/terraform plan \
  -var-file="environments/aws/demo.auto.tfvars.json" \
  -var-file="environments/azure/demo.auto.tfvars.json" \
  -var-file="environments/gcp/demo.auto.tfvars.json" \
  -var-file="environments/vnfs/demo.auto.tfvars.json" \
  -var-file="environments/network/demo.mesh.auto.tfvars.json" \
  -var-file="environments/dns/demo.auto.tfvars.json"
```

Confirm the plan includes three AWS regions, three Azure hubs, three GCP regions, and the on-prem VNFs (San Jose, Atlanta, Dubai).

## 2. Phase Apply Strategy

1. **Azure control plane** – build vWAN hubs, firewalls, NAT, and private endpoints.
2. **GCP regions** – create HA VPN gateways and workloads so AWS can consume the peer IPs.
3. **Full apply** – converge AWS (TGW, GWLB, EKS/RDS, Network Firewall) and wire the mesh.

Refer to the README for command snippets. Expect 25–35 minutes per full build (Azure/GCP VPNs dominate runtime).

## 3. Outputs & Artefacts

After `terraform apply`:

- `terraform output` surfaces:
  - `aws_transit_gateways`, `aws_network_manager`, `reachability` maps.
  - `azure_virtual_hubs`, `azure_firewalls`, Azure connection monitor IDs.
  - `gcp_ha_vpn_gateways`, `gcp_connectivity_tests`.
  - `vpn_endpoint_manifest` (sensitive) for Forward ingestion.
- `outputs/vpn-endpoints.json` – consolidated manifest of all cloud/VNF tunnels plus PSKs.
- `docs/demo-workflow.md` – regenerate with `scripts/generate_demo_doc.py` for live identifiers.

## 4. Forward Demo Path Catalog

| ID | Source → Destination | Protocol / Port | Key Controls |
|----|----------------------|-----------------|--------------|
| AWS-ALB-US-EAST | Internet → ALB us-east-1 → ASG → RDS | TCP 443 → HTTP 80 → TCP 5432 | ALB SG, ASG SG, data-tier NACL |
| AWS-GWLB | Internet → GWLB Endpoint → Palo Alto → EKS Service | TCP 443 → NodePort 30080/30443 | GWLB service, firewall bootstrap policies |
| AWS-TGW-Connect | Fortinet TGW Connect → TGW → Shared VPCs | GRE/BGP | TGW route tables, Fortinet configuration |
| Azure-FrontDoor | Azure Front Door → App Gateway → App Service | HTTPS 443 | WAF policy + hub route tables |
| Azure-Firewall | Shared VNet → Azure Firewall → spoke | TCP 443 | Firewall rules, NAT gateway override |
| GCP-Global-LB | HTTPS LB → Cloud Run | HTTPS 443 | Backend service, Cloud Armor (optional) |
| GCP-GKE | GKE Node → Logging VPC | TCP 6514 | Firewall rules, NAT |
| Mesh-AWS-Azure | TGW (us-east-1) → vWAN (uswest2) | IPsec/BGP | VPN manifests, route propagation |
| Mesh-AWS-GCP | TGW (us-east-1) → HA VPN (us-central1) | IPsec/BGP | Cloud Router, TGW VPN attachment |
| OnPrem-SJ → Azure | VNF San Jose → vWAN hub | IPsec/BGP | VNF manifest, Azure VPN site |
| OnPrem-Dubai → GCP | VNF Dubai → HA VPN | IPsec/BGP | VNF manifest, Cloud Router |

Use Forward Path Search to validate each path, then show Reachability Analyzer, Azure Connection Monitor, and GCP Connectivity Tests for native blind spots.

## 5. Troubleshooting Cheatsheet

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| AWS nodes fail to join EKS | IAM or security group rules missing | Run `terraform destroy` then re-apply once credentials/quotas confirmed. Ensure the `AmazonEKS*` policies are attached. |
| Palo Alto AMI lookup fails | Marketplace entitlement missing | Accept BYOL terms in the region or hardcode `gwlb_paloalto.ami_id`. |
| Azure Virtual WAN destroy hangs | Hub tear-down takes 10–15 minutes | Wait it out or delete hub + vWAN via `az network vhub/vwan delete`. |
| GCP network quota exceeded | `NETWORKS` limit (default 5) | Request quota increase or reduce regional VPC count. |

## 6. Cleanup

Destroy everything after the demo:

```bash
./bin/terraform destroy \
  -var-file="environments/aws/demo.auto.tfvars.json" \
  -var-file="environments/azure/demo.auto.tfvars.json" \
  -var-file="environments/gcp/demo.auto.tfvars.json" \
  -var-file="environments/vnfs/demo.auto.tfvars.json" \
  -var-file="environments/network/demo.mesh.auto.tfvars.json" \
  -var-file="environments/dns/demo.auto.tfvars.json"
```

Monitor Azure Virtual WAN hub deletion and confirm AWS TGW/VPN attachments disappear before re-running another demo.
