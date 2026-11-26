package installer

import "os"

var systemFiles = []struct {
	path     string
	content  string
	mode     os.FileMode
	preserve bool
}{
	{"/etc/caddy/posternproxy.json", `{"admin":{"listen":"127.0.0.1:2019"}}` + "\n", 0644, true},
	{"/etc/systemd/system/caddy.service", caddyUnit, 0644, false},
	{"/etc/systemd/system/posternproxy.service", controllerUnit, 0644, false},
	{"/etc/fail2ban/filter.d/posternproxy.conf", `[Definition]
failregex = ^.*"method":"POST","route":"/api/auth/login","client_ip":"<HOST>".*"status":401,
ignoreregex =
`, 0644, true},
	{"/etc/fail2ban/jail.d/posternproxy.conf", `[posternproxy]
enabled = true
filter = posternproxy
backend = polling
banaction = iptables-multiport
usedns = no
port = {{managementPort}}
logpath = /var/log/posternproxy/posternproxy.log
maxretry = 5
findtime = 60
bantime = 3600
`, 0644, true},
	{"/etc/apt/apt.conf.d/50unattended-upgrades-posternproxy", `Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
`, 0644, true},
	{"/etc/sysctl.d/99-posternproxy.conf", `net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv4.tcp_syncookies = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.conf.all.log_martians = 1
`, 0644, true},
}

const caddyUnit = `[Unit]
Description=Caddy Web Server
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
User=caddy
Group=caddy
SupplementaryGroups=posternproxy-certs
Environment=XDG_CONFIG_HOME=/var/lib/caddy
Environment=XDG_DATA_HOME=/var/lib/caddy
ExecStart=/usr/bin/caddy run --config /etc/caddy/posternproxy.json
Restart=on-failure
RestartSec=5
TimeoutStopSec=5s
LimitNOFILE=1048576
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/caddy
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateDevices=true
RestrictRealtime=true
RestrictSUIDSGID=true

[Install]
WantedBy=multi-user.target
`

const controllerUnit = `[Unit]
Description=PosternProxy Controller
After=network.target caddy.service
Wants=caddy.service

[Service]
Type=simple
User=posternproxy
Group=posternproxy-certs
EnvironmentFile=/etc/posternproxy/config.env
EnvironmentFile=-/run/posternproxy-install/admin.env
Environment=POSTERNPROXY_DATA_DIR=/var/lib/posternproxy
Environment=POSTERNPROXY_AGENT_BIN=/usr/local/bin/posternproxy-agent
Environment=POSTERNPROXY_LOG_FORMAT=json
Environment=POSTERNPROXY_CERT_RUNTIME_DIR=/run/posternproxy-certs
RuntimeDirectory=posternproxy-certs
RuntimeDirectoryMode=0750
RuntimeDirectoryPreserve=restart
WorkingDirectory=/var/lib/posternproxy
ExecStart=/usr/local/bin/posternproxy
Restart=always
RestartSec=5
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=/var/lib/posternproxy /var/log/posternproxy /run/posternproxy-certs
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
SystemCallFilter=@system-service
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
PrivateDevices=true
UMask=0077
StandardOutput=append:/var/log/posternproxy/posternproxy.log
StandardError=append:/var/log/posternproxy/posternproxy.log

[Install]
WantedBy=multi-user.target
`

const agentUnit = `[Unit]
Description=PosternProxy Agent
After=network-online.target caddy.service
Wants=network-online.target caddy.service

[Service]
Type=simple
User=posternproxy-agent
Group=posternproxy-certs
EnvironmentFile=/etc/posternproxy-agent/config.env
RuntimeDirectory=posternproxy-agent-certs
RuntimeDirectoryMode=0750
RuntimeDirectoryPreserve=restart
ExecStart=/usr/local/bin/posternproxy-agent
Restart=on-failure
RestartSec=5
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=/var/lib/posternproxy-agent /run/posternproxy-agent-certs
CapabilityBoundingSet=
PrivateDevices=true
RestrictSUIDSGID=true
UMask=0077

[Install]
WantedBy=multi-user.target
`
