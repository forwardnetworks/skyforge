#!/bin/bash
cat <<'EOF' >/var/log/skyforge-fortinet-bootstrap.conf
config system global
    set hostname ${hostname}
end

config system interface
    edit "port1"
        set mode static
        set ip ${inside_ip} ${inside_netmask}
    next
end

config router static
    edit 1
        set device "port1"
        set gateway ${tgw_inside_ip}
    next
end

config router bgp
    set as ${peer_bgp_asn}
    config neighbor
        edit "${tgw_inside_ip}"
            set activate enable
            set remote-as ${transit_gateway_asn}
            set ebgp-enforce-multihop enable
            set update-source "port1"
        next
    end
%{ if length(advertised_prefixes) > 0 }
    config network
%{ for idx, prefix in advertised_prefixes }
        edit ${idx + 1}
            set prefix ${prefix}
        next
%{ endfor }
    end
%{ endif }
end

config system admin
    edit "${admin_username}"
        set password "${admin_password}"
    next
end
EOF

logger "Skyforge Fortinet bootstrap rendered"
