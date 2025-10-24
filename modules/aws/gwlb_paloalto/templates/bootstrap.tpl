#cloud-config
write_files:
  - path: /config/init-cfg.txt
    permissions: '0644'
    owner: root:root
    content: |
      type=dhcp-client
%{ if auth_code != null }
      vm-auth-code=${auth_code}
%{ endif }
      hostname=${hostname}
      username=${admin_username}
      password=${admin_password}
  - path: /config/bootstrap.xml
    permissions: '0644'
    owner: root:root
    content: |
      <config>
        <shared>
          <log-settings>
            <profiles>
              <entry name="${log_profile}">
                <match-list>
                  <entry name="skyforge-default">
                    <send-to-panorama>no</send-to-panorama>
                    <send-to-syslog>yes</send-to-syslog>
                    <send-to-email>no</send-to-email>
                    <send-to-snmp>no</send-to-snmp>
                    <send-to-https>no</send-to-https>
                  </entry>
                </match-list>
              </entry>
            </profiles>
          </log-settings>
        </shared>
        <deviceconfig>
          <system>
            <hostname>${hostname}</hostname>
            <dns-setting>
              <servers>
%{ if length(dns_servers) > 0 ~}
                <primary>${dns_servers[0]}</primary>
%{ endif ~}
%{ if length(dns_servers) > 1 ~}
                <secondary>${dns_servers[1]}</secondary>
%{ endif ~}
              </servers>
            </dns-setting>
            <ntp-servers>
%{ if length(ntp_servers) > 0 ~}
              <primary>
                <ntp-server-address>${ntp_servers[0]}</ntp-server-address>
              </primary>
%{ endif ~}
%{ if length(ntp_servers) > 1 ~}
              <secondary>
                <ntp-server-address>${ntp_servers[1]}</ntp-server-address>
              </secondary>
%{ endif ~}
            </ntp-servers>
          </system>
        </deviceconfig>
        <vsys>
          <entry name="vsys1">
            <zone>
              <entry name="trust">
                <network>
                  <layer3>
                    <member>ethernet1/1</member>
                  </layer3>
                </network>
              </entry>
              <entry name="untrust">
                <network>
                  <layer3>
                    <member>ethernet1/2</member>
                  </layer3>
                </network>
              </entry>
              <entry name="inspection">
                <network>
                  <layer3>
                    <member>ethernet1/3</member>
                  </layer3>
                </network>
              </entry>
            </zone>
%{ if length(address_objects) > 0 ~}
            <address>
%{ for obj in address_objects ~}
              <entry name="${obj.name}">
%{ if obj.description != "" ~}
                <description>${obj.description}</description>
%{ endif ~}
%{ if obj.type == "ip-netmask" ~}
                <ip-netmask>${obj.value}</ip-netmask>
%{ else ~}
%{ if obj.type == "ip-range" ~}
                <ip-range>${obj.value}</ip-range>
%{ else ~}
%{ if obj.type == "fqdn" ~}
                <fqdn>${obj.value}</fqdn>
%{ else ~}
                <${obj.type}>${obj.value}</${obj.type}>
%{ endif ~}
%{ endif ~}
%{ endif ~}
              </entry>
%{ endfor ~}
            </address>
%{ endif ~}
%{ if length(service_objects) > 0 ~}
            <service>
%{ for svc in service_objects ~}
              <entry name="${svc.name}">
%{ if svc.description != "" ~}
                <description>${svc.description}</description>
%{ endif ~}
                <protocol>
                  <${lower(svc.protocol)}>
                    <port>${svc.destination_port}</port>
%{ if svc.source_port != "any" ~}
                    <source-port>${svc.source_port}</source-port>
%{ endif ~}
                  </${lower(svc.protocol)}>
                </protocol>
              </entry>
%{ endfor ~}
            </service>
%{ endif ~}
            <rulebase>
              <security>
%{ if length(security_policies) == 0 ~}
                <rules>
                  <entry name="skyforge-allow-trust-to-untrust">
                    <from>
                      <member>trust</member>
                    </from>
                    <to>
                      <member>untrust</member>
                    </to>
                    <source>
                      <member>any</member>
                    </source>
                    <destination>
                      <member>any</member>
                    </destination>
                    <service>
                      <member>application-default</member>
                    </service>
                    <application>
                      <member>any</member>
                    </application>
                    <action>allow</action>
                    <log-setting>${log_profile}</log-setting>
                  </entry>
                  <entry name="skyforge-block-untrust-to-trust">
                    <from>
                      <member>untrust</member>
                    </from>
                    <to>
                      <member>trust</member>
                    </to>
                    <source>
                      <member>any</member>
                    </source>
                    <destination>
                      <member>any</member>
                    </destination>
                    <service>
                      <member>application-default</member>
                    </service>
                    <application>
                      <member>any</member>
                    </application>
                    <action>deny</action>
                    <log-setting>${log_profile}</log-setting>
                  </entry>
                </rules>
%{ else ~}
                <rules>
%{ for rule in security_policies ~}
                  <entry name="${rule.name}">
                    <description>${rule.description}</description>
                    <from>
%{ for zone in rule.source_zones ~}
                      <member>${zone}</member>
%{ endfor ~}
                    </from>
                    <to>
%{ for zone in rule.destination_zones ~}
                      <member>${zone}</member>
%{ endfor ~}
                    </to>
                    <source>
%{ for addr in rule.source_addresses ~}
                      <member>${addr}</member>
%{ endfor ~}
                    </source>
                    <destination>
%{ for addr in rule.destination_addresses ~}
                      <member>${addr}</member>
%{ endfor ~}
                    </destination>
                    <service>
%{ for svc in rule.services ~}
                      <member>${svc}</member>
%{ endfor ~}
                    </service>
                    <application>
%{ for app in rule.applications ~}
                      <member>${app}</member>
%{ endfor ~}
                    </application>
                    <action>${rule.action}</action>
                    <log-setting>%{ if rule.log_setting != null }${rule.log_setting}%{ else }${log_profile}%{ endif }</log-setting>
                  </entry>
%{ endfor ~}
                </rules>
%{ endif ~}
              </security>
            </rulebase>
          </entry>
        </vsys>
      </config>

runcmd:
  - mkdir -p /config/logdb
  - touch /config/autocommit_on
  - echo "Skyforge Palo Alto bootstrap applied" >> /var/log/skyforge-bootstrap.log
