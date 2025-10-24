# Skyforge 1.0 – Multi-Cloud Demo Fabric

Skyforge is a Terraform-driven lab that stands up a Forward Networks showcase environment across AWS, Azure, GCP, and virtual network functions (VNFs). The 1.0 release delivers a three-region footprint per cloud, pre-built security stacks, reachability scenarios, and automation that publishes demo documentation.

- **AWS** – Transit Gateway mesh, Fortinet TGW Connect, Palo Alto Gateway Load Balancer, AWS Network Firewall, multi-tier app stacks (ALB + ASG + RDS), Network Manager inventory, Reachability Analyzer paths, and PrivateLink/NAT/peering catalog VPCs.
- **Azure** – Virtual WAN hubs, Azure Firewall, Cisco ASA NVAs, load balancers, NAT gateways, private endpoints, and connection monitors that mirror the AWS reachability checks.
- **GCP** – HA VPN sites, Cloud Router/NAT, regional workloads (GKE, Cloud Run, Cloud SQL, Pub/Sub), Check Point firewall option, and Connectivity Tests.
- **Shared/VNF** – Palo Alto and Fortinet VNFs with credential outputs and a manifest that Forward can ingest.

The repo is organised around a top-level `main.tf` orchestrator, provider-specific modules under `modules/`, and ready-to-use sample configurations under `environments/`.

```
skyforge/
├── main.tf                 # orchestrates cloud + mesh modules
├── environments/           # sample *.tfvars.json batches
├── modules/                # aws/, azure/, gcp/, shared/, vnfs/
├── scripts/                # demo doc generator + server
└── outputs/                # vpn-endpoints.json manifest target
```

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| Terraform CLI v1.6+ | Uses disallowing anonymous variables and provider version pins. |
| AWS CLI w/ SSO or keys | Use an AWS profile or `aws login`. |
| Azure CLI or service principal | Run `az login` and export `ARM_SUBSCRIPTION_ID`, `ARM_TENANT_ID`, and if needed `ARM_CLIENT_ID`/`ARM_CLIENT_SECRET`. |
| Google Cloud SDK | `gcloud auth application-default login` and `export GOOGLE_PROJECT=skyforge-475919` or set `GOOGLE_APPLICATION_CREDENTIALS`. |
| Terraform providers access | Ensure your account has access to AWS/Azure/GCP marketplace images (Fortinet/Palo Alto/Check Point require BYOL entitlements). |

> Copy `.env.local.sample` to `.env.local`, populate cloud credentials, then `source .env.local` before running any Terraform commands.

## Deployment Workflow

1. **Initialise Terraform**
   ```bash
   terraform init
   ```

2. **Plan per cloud** (safe blast radius)
   ```bash
   terraform plan -var-file="environments/aws/demo.auto.tfvars.json"
   terraform plan -var-file="environments/azure/demo.auto.tfvars.json"
   terraform plan -var-file="environments/gcp/demo.auto.tfvars.json"
   ```

3. **Layer mesh + VNFs + DNS when ready**
   ```bash
   terraform plan \
     -var-file="environments/aws/demo.auto.tfvars.json" \
     -var-file="environments/azure/demo.auto.tfvars.json" \
     -var-file="environments/gcp/demo.auto.tfvars.json" \
     -var-file="environments/vnfs/demo.auto.tfvars.json" \
     -var-file="environments/network/demo.mesh.auto.tfvars.json" \
     -var-file="environments/dns/demo.auto.tfvars.json"
   ```

4. **Recommended staged apply** (Azure/GCP public IPs are prerequisites for the AWS mesh):
   ```bash
   # Phase 1: Azure control plane
   terraform apply -target=module.azure_regions \
     -var-file="environments/aws/demo.auto.tfvars.json" \
     -var-file="environments/azure/demo.auto.tfvars.json" \
     -var-file="environments/gcp/demo.auto.tfvars.json" \
     -var-file="environments/vnfs/demo.auto.tfvars.json" \
     -var-file="environments/network/demo.mesh.auto.tfvars.json"

   # Phase 2: GCP regions (create Cloud VPN endpoints)
   terraform apply \
     -target=module.gcp_us_central1 \
     -target=module.gcp_europe_west1 \
     -target=module.gcp_asia_southeast1 \
     -var-file=... (same list as above)

   # Phase 3: Full convergence
   terraform apply \
     -var-file="environments/aws/demo.auto.tfvars.json" \
     -var-file="environments/azure/demo.auto.tfvars.json" \
     -var-file="environments/gcp/demo.auto.tfvars.json" \
     -var-file="environments/vnfs/demo.auto.tfvars.json" \
     -var-file="environments/network/demo.mesh.auto.tfvars.json" \
     -var-file="environments/dns/demo.auto.tfvars.json"
   ```

5. **Generate the Forward demo brief** (after a successful apply)
   ```bash
   ./scripts/generate_demo_doc.py --no-serve
   # or serve the doc at http://127.0.0.1:<random_port>
   ./scripts/generate_demo_doc.py
   ```

6. **Destroy after every demo**
   ```bash
   terraform destroy \
     -var-file="environments/aws/demo.auto.tfvars.json" \
     -var-file="environments/azure/demo.auto.tfvars.json" \
     -var-file="environments/gcp/demo.auto.tfvars.json" \
     -var-file="environments/vnfs/demo.auto.tfvars.json" \
     -var-file="environments/network/demo.mesh.auto.tfvars.json" \
     -var-file="environments/dns/demo.auto.tfvars.json"
   ```
   Azure Virtual WAN hubs can take 10–15 minutes to delete; watch `az network vhub show` if destroy is slow.

## Feature Highlights

### AWS
- Four dedicated VPCs (DMZ, shared-services, inspection, logging) per region with optional bastion/serverless/logging add-ons.
- Transit Gateway base mesh plus optional TGW Connect (Fortinet) and Global Accelerator.
- Palo Alto VM-Series firewalls behind Gateway Load Balancer, Network Firewall policies, NAT gateways, Interface VPC Endpoints, VPC peering, and managed prefix lists.
- App stack (ALB + ASG + RDS) and optional EKS cluster for advanced Forward Path Search demos.
- Network Manager global network, Reachability Analyzer paths, and metadata outputs to feed Forward ingestion.

### Azure
- Multi-region Virtual WAN hubs with Azure Firewall, NAT gateways, private endpoints, and VNet peering.
- Cisco ASA NVAs, Azure Application Gateway/App Service/Storage/SQL workloads with optional load balancers.
- Route tables for NVA chaining, Connection Monitor reachability tests, and structured outputs for Forward modeling.

### GCP
- Regional VPC blueprints with Cloud Router/NAT, HA VPN attachments, and optional Check Point VNFs.
- Workload modules for GKE, Cloud Run, Cloud SQL, Cloud Storage, Pub/Sub, and HTTP(S) global load balancing.
- Connectivity Tests mirroring AWS/Azure reachability scenarios.

### Shared / VNF
- `modules/shared/vnfs` emits `outputs/vpn-endpoints.json` describing every VPN link, PSK, and credential for ingestion.
- Sample VNFs in `environments/vnfs/demo.auto.tfvars.json` include Palo Alto (San Jose/Atlanta) and Fortinet peers.

### Demo Content Automation
- `scripts/generate_demo_doc.py` renders `docs/demo-workflow.md` from Terraform outputs and can host a local reference site.
- README sections link directly to environment files for quick customisation.

## Approximate Hourly Costs (On-Demand)

| Cloud | Regions | Major Resources | Approx. Hourly Cost* |
|-------|---------|-----------------|-----------------------|
| AWS | us-east-1, eu-central-1, ap-northeast-1 | TGW + VPN, NAT GW, ALB, 2× t3.micro ASG, RDS t3.micro, EKS control plane, Network Firewall, Palo Alto GWLB, Fortinet TGW Connect, interface endpoints | **~$3.30/hr** |
| Azure | uswest2, northeurope, japaneast | vWAN hubs + gateways, Azure Firewall, NAT gateway, ASA VM, Application Gateway/App Service/SQL/Storage, public IPs | **~$2.45/hr** |
| GCP | us-central1, europe-west1, asia-southeast1 | VPC stacks (Cloud Router/NAT/HA VPN), GKE (2× e2-standard-2), Cloud Run, Cloud SQL, Pub/Sub, Storage, optional Check Point VM | **~$1.80/hr** |
| VNFs & Misc | Palo Alto / Fortinet / Check Point appliances, Route 53 private DNS | **~$0.80/hr** |
| **Estimated Total** |  |  | **~$8.35/hr** |

\*Infrastructure-only estimates (compute, managed firewalls, load balancers, control planes). Data transfer, Marketplace licensing, idle IPs, and support contracts are excluded. Trim optional workloads (EKS, RDS, Check Point, etc.) if you need a smaller footprint.

## Usage Tips & Caveats

- **Marketplace AMIs** – Palo Alto and other third-party images require an accepted Marketplace agreement in each AWS region. If AMI lookup fails, set `gwlb_paloalto.ami_id` manually.
- **GCP VPC Quotas** – The sample config stays under the default `NETWORKS` quota (5) by limiting each region to a single VPC. Raise the quota if you add more VPCs.
- **Azure Hubs** – Deleting Virtual WAN hubs still takes time; destroy runs for ~12–15 minutes per hub. Plan demo windows accordingly.
- **Cleanup Discipline** – Destroy everything after every demo. The repo produces numerous managed services that bill hourly.
- **Forward Integration** – After apply, point Forward at `outputs/vpn-endpoints.json` and import the generated doc to walk customers through the scenarios. Once the model is ingested, destroy the lab (`terraform destroy ...`) to stop hourly spend.

## Next Steps

- Expand the mesh model for additional VNFs or Cloud-to-Cloud links.
- Automate CI checks (`terraform fmt`, `terraform validate`, `tflint`) and add Forward verification scripts.
- Enhance the cost calculator to pull live pricing via APIs or Terraform state.

Happy modeling! 🎯
