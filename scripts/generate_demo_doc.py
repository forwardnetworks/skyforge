#!/usr/bin/env python3
"""Generate the Skyforge demo workflow markdown using Terraform outputs.

This script can run offline; when state or outputs are unavailable it will
produce documentation that calls out the missing data and leaves placeholders
in the path catalog.
"""

from __future__ import annotations

import argparse
import datetime as _dt
import html
import json
import pathlib
import socket
import socketserver
import subprocess
from http import HTTPStatus
from typing import Any, Dict, Optional

import http.server

REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
DOC_PATH = REPO_ROOT / "docs" / "demo-workflow.md"
TF_CANDIDATES = (REPO_ROOT / "bin" / "terraform", pathlib.Path("terraform"))


def _run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, capture_output=True, text=True, check=False)


def _load_outputs() -> tuple[Optional[Dict[str, Any]], Optional[str]]:
    last_error: Optional[str] = None
    for tf_bin in TF_CANDIDATES:
        if tf_bin == pathlib.Path("terraform"):
            cmd = ["terraform", "output", "-json"]
        else:
            if not tf_bin.exists():
                continue
            cmd = [str(tf_bin), "output", "-json"]

        result = _run(cmd)
        if result.returncode == 0:
            try:
                return json.loads(result.stdout), None
            except json.JSONDecodeError as exc:  # pragma: no cover - defensive
                last_error = f"Failed to decode terraform output: {exc}"
        else:
            stderr = result.stderr.strip()
            last_error = stderr or "terraform output failed"
    return None, last_error


def _unwrap(outputs: Dict[str, Any], key: str, default: Any = None) -> Any:
    if not outputs or key not in outputs:
        return default
    value_block = outputs[key]
    if isinstance(value_block, dict) and "value" in value_block:
        return value_block["value"]
    return value_block


def _format_path_table(context: Dict[str, Any]) -> str:
    aggregator = context.get("global_accelerator_dns")
    albs: Dict[str, Dict[str, Any]] = context.get("application_albs", {})

    def alb_dns(region: str) -> str:
        entry = albs.get(region)
        dns = entry.get("dns_name") if entry else None
        return dns or f"ALB {region} (pending)"

    flows = [
        {
            "name": "Global App Ingress (us-east-1)",
            "source": aggregator or "Internet client",
            "destination": f"{alb_dns('us-east-1')} → Web ASG → PostgreSQL",  # noqa: E501
            "protocol": "TCP 80/443 → backend 80 → TCP 5432",
            "focus": "Forward Path Search – multi-tier reachability, SGs, NACL",  # noqa: E501
        },
        {
            "name": "Global App Ingress (eu-central-1)",
            "source": aggregator or "Internet client",
            "destination": f"{alb_dns('eu-central-1')} → Web ASG → Network Firewall → PostgreSQL",  # noqa: E501
            "protocol": "TCP 80/443 → backend 80 → TCP 5432",
            "focus": "Forward Path Search – AWS Network Firewall inspection",  # noqa: E501
        },
        {
            "name": "Global App Ingress (ap-northeast-1)",
            "source": aggregator or "Internet client",
            "destination": f"{alb_dns('ap-northeast-1')} → GWLB → Palo Alto → EKS service",  # noqa: E501
            "protocol": "TCP 80/443 → NodePort 30080/30443",
            "focus": "Forward Path Search – GWLB inline firewall + Kubernetes",  # noqa: E501
        },
        {
            "name": "DMZ to App (us-east-1)",
            "source": "DMZ frontend subnet",
            "destination": "Shared-services app subnet",
            "protocol": "TCP 8080/8443 (ephemeral return)",
            "focus": "Security group `skyforge-us-east-1-app-sg` + NACL `dmz-frontend-web`",  # noqa: E501
        },
        {
            "name": "Application to Database",
            "source": "Shared-services app subnet",
            "destination": "Shared-services data subnet",
            "protocol": "TCP 5432",
            "focus": "Forward Path Search – Data-tier ACL `data-tier` enforcement",  # noqa: E501
        },
        {
            "name": "Logging Ingest Pipeline",
            "source": "App/DMZ tier",
            "destination": "Logging VPC ingest subnet",
            "protocol": "TCP 6514, UDP 514",
            "focus": "Security group `logging-ingest` + ACL `logging-ingress-acl`",  # noqa: E501
        },
        {
            "name": "Lambda Controlled Egress",
            "source": "Serverless private subnet",
            "destination": "Internet + internal APIs",
            "protocol": "TCP 443, TCP 8443",
            "focus": "Security group `lambda-egress` + NAT/TGW routing",
        },
        {
            "name": "Bastion Administration",
            "source": "Corporate IPs",
            "destination": "Bastion hosts",
            "protocol": "TCP 22 / TCP 3389",
            "focus": "Security group `bastion-admin`, ACL `bastion-acl`",
        },
        {
            "name": "GWLB Inline Inspection",
            "source": "Internet client",
            "destination": "Palo Alto → app subnet",
            "protocol": "TCP 80/443",
            "focus": "GWLB endpoint chaining + security group `gwlb-management`",
        },
        {
            "name": "Transit Gateway Connect Overlay",
            "source": "Fortinet FortiGate (us-east-1 transport)",
            "destination": "AWS Transit Gateway Connect attachment",
            "protocol": "GRE 47 + BGP TCP 179",
            "focus": "Forward Path Search – TGW Connect GRE overlay and BGP route advertisement",
        },
        {
            "name": "PrivateLink SSM Access",
            "source": "Shared-services app subnet",
            "destination": "SSM interface endpoint",
            "protocol": "TCP 443",
            "focus": "Forward Path Search – PrivateLink interface endpoint coverage",
        },
        {
            "name": "San Jose On-Prem → Azure",
            "source": "VNF San Jose",
            "destination": "Azure vWAN hub (uswest2)",
            "protocol": "IPsec/BGP, tunneled HTTPS",
            "focus": "Forward Path Search – VPN manifest, TGW → vWAN connectivity",
        },
        {
            "name": "Dubai On-Prem → GCP",
            "source": "VNF Dubai",
            "destination": "GCP HA VPN (us-central1)",
            "protocol": "IPsec/BGP, tunneled HTTPS",
            "focus": "Forward Path Search – Multi-cloud VPN routing",
        },
        {
            "name": "Azure Front Door Path",
            "source": "Azure Front Door endpoint",
            "destination": "Azure Application Gateway → App Service",
            "protocol": "HTTPS 443 (health probes HTTP)",
            "focus": "Forward Path Search – Azure WAF policy + hub reachability",
        },
        {
            "name": "GCP Global HTTP LB",
            "source": "GCP HTTPS LB",
            "destination": "Cloud Run service",
            "protocol": "HTTPS 443",
            "focus": "Forward Path Search – GCP firewall rules + Cloud Run IAM",
        },
        {
            "name": "Blocked Path Sanity Check",
            "source": "Bastion subnet",
            "destination": "Logging ops subnet",
            "protocol": "TCP 22",
            "focus": "Forward Path Search – detect intentional violation (expect DENY)",  # noqa: E501
        },
    ]

    rows = ["| Flow | Source → Destination | Protocols / Ports | Forward Path Search Focus |"]
    rows.append("|------|---------------------|--------------------|------------------------------|")
    for flow in flows:
        source = flow["source"]
        destination = flow.get("destination", "")
        rows.append(
            f"| {flow['name']} | {source} → {destination} | {flow['protocol']} | {flow['focus']} |"
        )
    return "\n".join(rows)


def render(outputs: Optional[Dict[str, Any]], error: Optional[str]) -> str:
    timestamp = _dt.datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S UTC")

    multi_lb = _unwrap(outputs or {}, "multi_cloud_load_balancing", {}) if outputs else {}
    reachability = _unwrap(outputs or {}, "reachability", {}) if outputs else {}
    vpn_manifest = _unwrap(outputs or {}, "vpn_endpoint_manifest", {}) if outputs else {}
    clouds = vpn_manifest.get("clouds", {}) if isinstance(vpn_manifest, dict) else {}
    aws_stack = {}
    if isinstance(clouds, dict):
        aws_cloud = clouds.get("aws", {}) or {}
        if isinstance(aws_cloud, dict):
            for region, data in aws_cloud.items():
                if isinstance(data, dict):
                    aws_stack[region] = data.get("application_stack") or {}

    application_albs = multi_lb.get("application_albs") or {}
    for region, info in application_albs.items():
        if isinstance(info, dict):
            application_albs[region] = {
                "dns_name": info.get("dns_name"),
                "zone_id": info.get("zone_id"),
            }
    aws_block = multi_lb.get("aws", {}) if isinstance(multi_lb, dict) else {}
    azure_block = multi_lb.get("azure", {}) if isinstance(multi_lb, dict) else {}
    gcp_block = multi_lb.get("gcp", {}) if isinstance(multi_lb, dict) else {}

    global_app = aws_block.get("global_application_accelerator") or {}
    accelerator_dns = None
    if isinstance(global_app, dict):
        accelerator_dns = global_app.get("custom_domain") or global_app.get("dns_name")

    context = {
        "global_accelerator_dns": accelerator_dns,
        "global_accelerator_alias": global_app.get("custom_domain") if isinstance(global_app, dict) else None,
        "global_accelerator_listener_ports": global_app.get("listener_ports") if isinstance(global_app, dict) else [],
        "application_albs": application_albs,
        "tgw_connect": aws_block.get("transit_gateway_connect", {}),
        "gwlb": aws_block.get("gateway_load_balancers", {}),
        "azure_asa": azure_block.get("asa", {}),
        "gcp_checkpoint": gcp_block.get("checkpoint_firewalls", {}),
        "aws_stack": aws_stack,
        "reachability": reachability,
    }

    path_table = _format_path_table(context)

    accelerator_label = "the accelerator DNS name"
    if accelerator_dns:
        accelerator_label = f"`{accelerator_dns}`"
    accelerator_alias = context.get("global_accelerator_alias")
    if accelerator_alias:
        accelerator_label = f"`{accelerator_alias}` (alias for `{accelerator_dns}`)"
    ga_ports = context.get("global_accelerator_listener_ports") or []
    ports_note = ""
    if ga_ports:
        ports_note = f" on listener ports {', '.join(str(p) for p in ga_ports)}"

    outputs_section = "Terraform outputs available." if outputs else "Terraform outputs unavailable (run `terraform apply` to populate dynamic values)."
    if error:
        outputs_section = f"Terraform outputs unavailable: {error.strip()}"

    def _render_credentials(creds: Optional[Dict[str, Any]]) -> str:
        if not isinstance(creds, dict):
            return "user n/a / pass n/a"
        username = creds.get("username") or "n/a"
        password = creds.get("password") or "n/a"
        return f"user {username} / pass {password}"

    security_lines: list[str] = []

    tgw_connect = context.get("tgw_connect") or {}
    if isinstance(tgw_connect, dict) and tgw_connect:
        security_lines.append("#### AWS Transit Gateway Connect (Fortinet)")
        for region in sorted(tgw_connect.keys()):
            entry = tgw_connect.get(region) or {}
            connector = entry.get("connector") or {}
            mgmt_ip = connector.get("management_ip") or connector.get("private_ip") or "pending"
            creds = _render_credentials(connector.get("admin_credentials"))
            route_bits: list[str] = []
            if entry.get("inspection_route_table_id"):
                route_bits.append(f"inspection RT `{entry['inspection_route_table_id']}`")
            if entry.get("appliance_route_table_id"):
                route_bits.append(f"appliance RT `{entry['appliance_route_table_id']}`")
            route_note = f" ({', '.join(route_bits)})" if route_bits else ""
            security_lines.append(f"- **{region}** — mgmt IP `{mgmt_ip}`{route_note} ({creds})")

    gwlb = context.get("gwlb") or {}
    if isinstance(gwlb, dict) and gwlb:
        security_lines.append("\n#### AWS GWLB Palo Alto")
        for region in sorted(gwlb.keys()):
            entry = gwlb.get(region) or {}
            firewalls = entry.get("firewalls") or {}
            private_ips = firewalls.get("private_ips") or []
            ip_display = ", ".join(private_ips) if private_ips else "pending"
            creds = _render_credentials(firewalls.get("admin_credentials"))
            security_lines.append(f"- **{region}** — firewalls `{ip_display}` ({creds})")

    azure_asa = context.get("azure_asa") or {}
    if isinstance(azure_asa, dict) and any(azure_asa.values()):
        security_lines.append("\n#### Azure ASA NVAs")
        for region in sorted(azure_asa.keys()):
            info = azure_asa.get(region)
            if not info:
                continue
            private_ip = info.get("private_ip") or "pending"
            public_ip = info.get("public_ip") or "n/a"
            creds = _render_credentials(
                {
                    "username": info.get("admin_username"),
                    "password": info.get("admin_password"),
                }
            )
            security_lines.append(f"- **{region}** — private `{private_ip}` / public `{public_ip}` ({creds})")

    gcp_checkpoint = context.get("gcp_checkpoint") or {}
    if isinstance(gcp_checkpoint, dict) and any(gcp_checkpoint.values()):
        security_lines.append("\n#### GCP Check Point Firewalls")
        for region in sorted(gcp_checkpoint.keys()):
            info = gcp_checkpoint.get(region)
            if not info:
                continue
            private_ip = info.get("private_ip") or "pending"
            creds = _render_credentials(
                {
                    "username": info.get("admin_username"),
                    "password": info.get("admin_password"),
                }
            )
            security_lines.append(f"- **{region}** — private `{private_ip}` ({creds})")

    security_section = "\n".join(security_lines) if security_lines else "Security appliance outputs unavailable."

    appliance_inventory: list[str] = []

    default_fortinet_name = "skyforge-us-east-1-fortinet"
    default_paloalto_name = "skyforge-us-east-1-gwlb"
    default_asa_name = "vm-skyforge-uswest2-asa"
    default_checkpoint_name = "cp-us-central1-firewall"

    if isinstance(context.get("tgw_connect"), dict) and context["tgw_connect"]:
        fortinet_name = default_fortinet_name
        fortinet = context["tgw_connect"].get("us-east-1") or {}
        instance = fortinet.get("connector", {}).get("instance_id") or fortinet_name
        appliance_inventory.append(f"- **Fortinet TGW Connect** (`{instance}`) — management IP `{fortinet.get('connector', {}).get('management_ip', 'pending')}`")
    else:
        appliance_inventory.append(f"- **Fortinet TGW Connect** (`{default_fortinet_name}`) — deploy via `transit_gateway_connect.connector`")

    gwlb = context.get("gwlb") or {}
    if isinstance(gwlb, dict) and gwlb:
        palo = gwlb.get("ap-northeast-1") or gwlb.get("us-east-1") or {}
        palo_instances = ", ".join(palo.get("firewalls", {}).get("private_ips", []) or []) or "pending"
        appliance_inventory.append(f"- **Palo Alto GWLB** (`{default_paloalto_name}`) — endpoint IPs `{palo_instances}`")
    else:
        appliance_inventory.append(f"- **Palo Alto GWLB** (`{default_paloalto_name}`) — enable `enable_gateway_lb` in AWS regions")

    asa_map = context.get("azure_asa") or {}
    if isinstance(asa_map, dict) and asa_map:
        asa = asa_map.get("uswest2") or next((asa_map[k] for k in sorted(asa_map) if asa_map[k]), None)
        if asa:
            appliance_inventory.append(f"- **Azure ASA** (`{asa.get('vm_id', default_asa_name)}`) — private `{asa.get('private_ip', 'pending')}` public `{asa.get('public_ip', 'n/a')}`")
    else:
        appliance_inventory.append(f"- **Azure ASA** (`{default_asa_name}`) — configure `asa_nva` per region")

    checkpoint_map = context.get("gcp_checkpoint") or {}
    if isinstance(checkpoint_map, dict) and checkpoint_map:
        cp = checkpoint_map.get("us-central1") or next((checkpoint_map[k] for k in sorted(checkpoint_map) if checkpoint_map[k]), None)
        if cp:
            appliance_inventory.append(f"- **GCP Check Point** (`{cp.get('instance_id', default_checkpoint_name)}`) — private `{cp.get('private_ip', 'pending')}`")
    else:
        appliance_inventory.append(f"- **GCP Check Point** (`{default_checkpoint_name}`) — set `checkpoint_firewall` in GCP region config")

    appliance_section = "\n".join(appliance_inventory)

    reachability_lines: list[str] = []
    if isinstance(reachability, dict) and reachability:
        aws_reach = reachability.get("aws", {}) if isinstance(reachability.get("aws", {}), dict) else {}
        if aws_reach:
            aws_paths = aws_reach.get("paths", {}) or {}
            aws_analysis = aws_reach.get("analyses", {}) or {}
            if aws_paths or aws_analysis:
                reachability_lines.append("#### AWS Reachability Analyzer")
                if isinstance(aws_paths, dict) and aws_paths:
                    for region in sorted(aws_paths.keys()):
                        names = ", ".join(sorted(aws_paths[region].keys())) if isinstance(aws_paths[region], dict) else "--"
                        reachability_lines.append(f"- **{region}** paths: {names}")
                if isinstance(aws_analysis, dict) and aws_analysis:
                    for region in sorted(aws_analysis.keys()):
                        names = ", ".join(sorted(aws_analysis[region].keys())) if isinstance(aws_analysis[region], dict) else "--"
                        reachability_lines.append(f"  - Analyses: {names}")
        azure_reach = reachability.get("azure", {})
        if isinstance(azure_reach, dict) and azure_reach:
            reachability_lines.append("\n#### Azure Network Watcher")
            for region in sorted(azure_reach.keys()):
                monitor_names = ", ".join(sorted(azure_reach[region].keys())) if isinstance(azure_reach[region], dict) else "--"
                reachability_lines.append(f"- **{region}** monitors: {monitor_names}")
        gcp_reach = reachability.get("gcp", {})
        if isinstance(gcp_reach, dict) and gcp_reach:
            reachability_lines.append("\n#### GCP Connectivity Tests")
            for region in sorted(gcp_reach.keys()):
                test_names = ", ".join(sorted(gcp_reach[region].keys())) if isinstance(gcp_reach[region], dict) else "--"
                reachability_lines.append(f"- **{region}** tests: {test_names}")

    reachability_section = "\n".join(reachability_lines) if reachability_lines else "Reachability outputs unavailable."

    validation_section = """1. `./bin/terraform fmt` (runs via pre-commit)
2. `./bin/terraform validate`
3. `./bin/terraform plan -var-file=...` for each environment
4. `./bin/terraform apply -var-file=...` (staged per cloud if required)
5. `./bin/terraform destroy -var-file=...`
6. `python3 scripts/generate_demo_doc.py --no-serve`
7. Run Forward reachability tests against published path catalog"""

    return f"""# Skyforge Demo Workflow

> Generated by `scripts/generate_demo_doc.py` on {timestamp}.
> {outputs_section}

## 1. Bootstrap & Planning

```bash
source .env.local
./bin/terraform init
./bin/terraform plan \\
  -var-file="environments/aws/demo.auto.tfvars.json" \\
  -var-file="environments/azure/demo.auto.tfvars.json" \\
  -var-file="environments/gcp/demo.auto.tfvars.json" \\
  -var-file="environments/vnfs/demo.auto.tfvars.json" \\
  -var-file="environments/network/demo.mesh.auto.tfvars.json"
```

Review the plan output to confirm:

- AWS builds three application regions (us-east-1, eu-central-1, ap-northeast-1) with VPCs for shared services, DMZ, bastion, logging, and serverless workloads.
- TGW mesh peering connects the regions and TGW Connect is provisioned in us-east-1.
- Azure and GCP stacks are staged for mesh integration.
- The VNF manifest generates VPN connectivity for the on-prem sites.

## 2. Apply the Demo Environment

```bash
./bin/terraform apply -auto-approve \\
  -var-file="environments/aws/demo.auto.tfvars.json" \\
  -var-file="environments/azure/demo.auto.tfvars.json" \\
  -var-file="environments/gcp/demo.auto.tfvars.json" \\
  -var-file="environments/vnfs/demo.auto.tfvars.json" \\
  -var-file="environments/network/demo.mesh.auto.tfvars.json"
```

Expect the apply to take 20–30 minutes depending on quotas and regional capacity. Key components include:

- Three-tier application stacks (ALB/ASG/EKS/RDS) in each AWS region.
- Regional security constructs (TGW mesh, Network Firewall, GWLB/Palo Alto).
- Additional security groups, network ACLs, and subnet tiers in the bastion/logging/serverless VPCs.
- Azure vWAN + workloads and GCP HA VPN + workloads for cross-cloud paths.
- Updated VNF manifest in `outputs/vpn-endpoints.json`.

## 3. Post-Deploy Validation

1. **Check Terraform outputs**
   ```bash
   ./bin/terraform output -json | jq '.multi_cloud_load_balancing'
   ./bin/terraform output vpn_endpoint_manifest | jq '.'
   ```
2. **Application reachability**
   - Resolve the DNS names for each regional ALB (returned in the `application_albs` output).
- If Global Accelerator is enabled, test {accelerator_label}{ports_note} and confirm traffic fails over when you stop an ALB or GWLB endpoint.
3. **Security posture**
   - Inspect the security groups (`bastion-admin`, `logging-ingest`, `lambda-egress`, `gwlb-management`, etc.) and the network ACLs generated for DMZ/data/logging tiers.
   - Check TGW peering attachments and route tables to ensure each region advertises the new VPC CIDRs.
4. **Cross-cloud mesh**
   - Review Azure vWAN connections and GCP HA VPN tunnels to confirm they picked up the new mesh links.
5. **VNF inventory**
   - Inspect `outputs/vpn-endpoints.json` for generated PSKs, tunnel metadata, and on-prem connectivity entries for San Jose, Atlanta, and Dubai sites.

## 4. Demo Scenarios

Use the environment to highlight typical Forward Networks analyses:

### L4–L7 Path Catalog (use Forward Path Search)

{path_table}

### Security Appliance Credentials

{security_section}

### Appliance Inventory (Names & Tags)

{appliance_section}

### Reachability Outputs

{reachability_section}

### Validation Checklist

{validation_section}

- **Security group & ACL audits** – verify the bastion/logging/serverless security policies and the DMZ/data network ACLs.
- **Firewall policy review** – inspect Palo Alto address/service objects, AWS Network Firewall Suricata rules, and Azure Firewall collections for the expected demo flows.
- **TGW mesh visualisation** – confirm the full-mesh peering and on-prem VPN stubs.
- **Cross-cloud mesh checks** – validate AWS↔Azure and AWS↔GCP paths using the generated VPN credentials.

## 5. Teardown

Always destroy the environment after the demo to minimise cloud spend:

```bash
./bin/terraform destroy -auto-approve \\
  -var-file="environments/aws/demo.auto.tfvars.json" \\
  -var-file="environments/azure/demo.auto.tfvars.json" \\
  -var-file="environments/gcp/demo.auto.tfvars.json" \\
  -var-file="environments/vnfs/demo.auto.tfvars.json" \\
  -var-file="environments/network/demo.mesh.auto.tfvars.json"
```

Validate in AWS, Azure, and GCP consoles that resources have been deleted (especially TGW peering attachments and Global Accelerator endpoints, which can take several minutes to release).

---

Keep this document alongside the Terraform configs so the demo workflow stays consistent with future changes. Update it as you introduce new regions, workloads, or validation steps – or simply rerun `python scripts/generate_demo_doc.py` after each deployment.
"""


def _start_server(host: str, doc_path: pathlib.Path) -> None:
    class RequestHandler(http.server.BaseHTTPRequestHandler):
        def do_GET(self) -> None:  # noqa: N802
            if self.path not in ("/", "/index.html"):
                self.send_error(HTTPStatus.NOT_FOUND)
                return
            try:
                content = doc_path.read_text(encoding="utf-8")
            except FileNotFoundError:
                content = "# documentation not found\n"
            body = f"<html><body><pre>{html.escape(content)}</pre></body></html>".encode("utf-8")
            self.send_response(HTTPStatus.OK)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: Any) -> None:  # noqa: A003
            return

    with socketserver.TCPServer((host, 0), RequestHandler) as httpd:
        port = httpd.server_address[1]
        addr = f"http://{host}:{port}/"
        print(f"Serving demo workflow at {addr} (Ctrl+C to stop)")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nStopping documentation server")


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate demo doc and optionally serve it")
    parser.add_argument("--no-serve", action="store_true", help="Do not launch the local documentation server")
    parser.add_argument("--host", default="127.0.0.1", help="Host interface for the local server (default: 127.0.0.1)")
    args = parser.parse_args()

    outputs, error = _load_outputs()
    content = render(outputs, error)
    DOC_PATH.write_text(content, encoding="utf-8")
    print(f"Updated {DOC_PATH.relative_to(REPO_ROOT)}")

    if not args.no_serve:
        host = args.host
        if host == "0.0.0.0":
            try:
                hostname = socket.gethostname()
                host = socket.gethostbyname(hostname)
            except OSError:
                host = "0.0.0.0"
        _start_server(host, DOC_PATH)


if __name__ == "__main__":
    main()
