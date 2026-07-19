const accessMethods = ["Open ESXi", "Console"];

const linuxHardeningSteps = [
  "Set a safe PATH/EDITOR, identify the distro, and confirm DNS/NTP before making changes.",
  "Change root and privileged account passwords, create the metro working account, then validate sudo.",
  "Review sudoers, UID 0 accounts, logged-in users, root history, cron, and hidden root profile files.",
  "Lock root login, harden sshd_config, add the authorized banner, and validate sshd -T before restart.",
  "Baseline listening ports, choose the distro firewall path, and allow only mission-required traffic.",
  "Run malware/system checks, fix key file permissions, review application accounts, then patch if scoring allows."
];

const windowsHardeningSteps = [
  "Identify the Windows role, record IP/DNS/time, and confirm whether the host is domain joined.",
  "Change local Administrator and service-account passwords, then audit local Administrators membership.",
  "Review enabled users, Domain Admins where applicable, scheduled tasks, shares, and startup persistence.",
  "Enable Windows Defender Firewall and restrict RDP, SMB, web, FTP, DNS, and AD ports by host role.",
  "Enable security auditing, inspect Event Viewer, verify Defender status, and remove unused roles/services.",
  "Apply Windows updates in controlled passes, reboot when required, and recheck scoring-critical services."
];

const linuxHardeningGuide = [
  {
    id: "accounts-ssh",
    title: "Accounts & SSH",
    summary: "Start here. This content is distilled from the account and SSH section of kill_chain_checklist-Oct2026.md.",
    steps: [
      {
        title: "Set a safe shell environment",
        detail: "Normalize PATH first so aliases or poisoned shell paths do not influence the first response actions.",
        commands: [
          "export PATH=/bin:/sbin:/usr/bin:/usr/sbin",
          "export EDITOR=nano",
          "env | egrep 'EDITOR|PATH'",
          "cat /etc/os-release",
          "uname -a"
        ]
      },
      {
        title: "Change privileged passwords and create metro",
        detail: "Change root first, then create the working admin account used for the rest of the hardening work.",
        commands: [
          "passwd root",
          "passwd sysadmin",
          "groupadd -g 999999 metro",
          "useradd -u 999999 -g 999999 -s/bin/bash -c\"METRO\" -m metro",
          "passwd metro",
          "visudo -f /etc/sudoers.d/metro"
        ]
      },
      {
        title: "Validate sudo and lock root",
        detail: "Confirm metro can sudo before locking root. Keep an existing session open until SSH is verified.",
        commands: [
          "su - metro",
          "sudo -l",
          "sudo usermod -L root",
          "sudo grep -v '^#' /etc/sudoers /etc/sudoers.d/* | less"
        ]
      },
      {
        title: "Audit accounts, login files, and cron",
        detail: "Investigate privileged accounts, UID 0 accounts, hidden root login files, user history, and persistence.",
        commands: [
          "who",
          "last > ~/last_users.txt",
          "sudo cat /root/.bash_history > ~/root_history.txt",
          "sudo egrep -v \"root|nologin|false|halt|shutdown|sync\" /etc/passwd | awk -F: '($3 <= 1000) {print}'",
          "sudo egrep -v \"^\\+\" /etc/passwd | awk -F: '($3 >= 1000) {print}'",
          "sudo su - root -c 'crontab -l'"
        ]
      },
      {
        title: "Harden SSH and banner",
        detail: "Back up sshd_config, disable dangerous SSH behavior, add the banner, and validate before restarting.",
        commands: [
          "sudo cp /etc/ssh/sshd_config /etc/ssh/sshd_config.prev",
          "sudo chown root:root /etc/ssh/sshd_config",
          "sudo chmod 644 /etc/ssh/sshd_config",
          "sudo vi /etc/ssh/sshd_config",
          "sudo sshd -T | less",
          "sudo systemctl restart sshd"
        ]
      }
    ]
  },
  {
    id: "firewall-network",
    title: "Firewall & Network",
    summary: "Pick ufw, firewalld, or iptables based on the distro. Start with visibility, then restrict inbound access.",
    steps: [
      {
        title: "Baseline listening ports",
        detail: "Identify every listening service before changing firewall rules so scoring services are not blocked accidentally.",
        commands: [
          "ss -tulpan | less",
          "netstat -antup | less",
          "sudo cat /etc/resolv.conf",
          "date"
        ]
      },
      {
        title: "Detect firewall stack",
        detail: "Use the firewall system already present on the host whenever possible.",
        commands: [
          "sudo /bin/firewall-cmd --state",
          "sudo /usr/sbin/ufw status verbose",
          "sudo /sbin/iptables -L",
          "sudo /usr/sbin/nft list ruleset"
        ]
      },
      {
        title: "Ubuntu or Debian ufw path",
        detail: "Enable ufw, allow loopback, review numbered rules, and allow only required services from trusted ranges.",
        commands: [
          "sudo /usr/sbin/ufw enable",
          "sudo /usr/sbin/ufw status numbered",
          "sudo /usr/sbin/ufw allow in on lo",
          "sudo /usr/sbin/ufw allow out on lo",
          "sudo /usr/sbin/ufw deny in from 127.0.0.0/8",
          "sudo /usr/sbin/ufw reload"
        ]
      },
      {
        title: "Enterprise Linux firewalld path",
        detail: "Use the public zone unless the host role needs a stricter zone, then persist changes after validation.",
        commands: [
          "sudo /bin/firewall-cmd --get-default-zone",
          "sudo /bin/firewall-cmd --get-active-zones",
          "sudo /bin/firewall-cmd --set-default-zone=public",
          "sudo /bin/firewall-cmd --list-all --zone=public",
          "sudo /bin/firewall-cmd --runtime-to-permanent",
          "sudo /bin/firewall-cmd --reload"
        ]
      },
      {
        title: "Network sysctl hardening",
        detail: "Disable forwarding and redirects, then reload sysctl. Use the distro values that match the host.",
        commands: [
          "sudo cp /etc/sysctl.conf /etc/sysctl.conf.prev",
          "sudo vi /etc/sysctl.conf",
          "sudo sysctl --system"
        ]
      }
    ]
  },
  {
    id: "malware",
    title: "Malware Removal",
    summary: "Use package-native tools when available and preserve logs for review.",
    steps: [
      {
        title: "Run Linux audit tooling",
        detail: "Lynis, chkrootkit, and rkhunter provide quick visibility. Install from trusted repos only.",
        commands: [
          "sudo lynis audit system",
          "sudo /usr/sbin/chkrootkit",
          "sudo /usr/bin/rkhunter -c"
        ]
      },
      {
        title: "Install tools on Enterprise Linux if needed",
        detail: "Enable EPEL only when the host repo path is trusted and the scoring requirements permit it.",
        commands: [
          "sudo yum install epel-release",
          "sudo yum install rkhunter lynis",
          "sudo lynis audit system",
          "sudo /usr/bin/rkhunter -c"
        ]
      },
      {
        title: "Review persistence locations",
        detail: "After scanner output, inspect cron, systemd, rc scripts, and custom software paths manually.",
        commands: [
          "sudo su -c 'cd /var/spool/cron && ls -la'",
          "sudo find /etc/systemd/system /etc/systemd/user -maxdepth 2 -type f -ls",
          "sudo find /usr/local/bin /opt -maxdepth 2 -type f -ls"
        ]
      }
    ]
  },
  {
    id: "system-hardening",
    title: "System Hardening",
    summary: "Fix permissions, password policy, name resolution, and unnecessary services after access is stable.",
    steps: [
      {
        title: "Protect critical files",
        detail: "Correct ownership and permissions on passwd, group, shadow, gshadow, cron, and bootloader files.",
        commands: [
          "sudo chown root:root /etc/passwd /etc/group /etc/shadow",
          "sudo chmod 644 /etc/passwd /etc/group",
          "sudo chmod 600 /etc/shadow",
          "sudo chmod 600 /etc/gshadow",
          "sudo chown root:root /etc/crontab /etc/cron.d",
          "sudo chmod og-rwx /etc/crontab /etc/cron.d"
        ]
      },
      {
        title: "Check poisoned name service files",
        detail: "Hosts, resolv.conf, and nsswitch.conf can redirect traffic or authentication paths.",
        commands: [
          "sudo cat /etc/hosts",
          "sudo cat /etc/resolv.conf",
          "sudo cat /etc/nsswitch.conf"
        ]
      },
      {
        title: "Disable unused services",
        detail: "Stop and mask only services that are not required for the host mission.",
        commands: [
          "ps -ef | less",
          "sudo systemctl status <servicename>",
          "sudo systemctl --now mask <servicename>"
        ]
      },
      {
        title: "Enforce password policy",
        detail: "Use PAM or pwquality settings appropriate to the distro. Back up files before editing.",
        commands: [
          "sudo cp /etc/pam.d/common-password /etc/pam.d/common-password.prev",
          "sudo cp /etc/security/pwquality.conf /etc/security/pwquality.conf.prev",
          "sudo vi /etc/security/pwquality.conf"
        ]
      }
    ]
  },
  {
    id: "application",
    title: "Application",
    summary: "Change application passwords, review app roles, and fix ownership without breaking scoring services.",
    steps: [
      {
        title: "Inventory application accounts",
        detail: "Find service users, application admins, and any account with elevated application roles.",
        commands: [
          "sudo egrep -v \"nologin|false\" /etc/passwd",
          "sudo find /opt /var/www /etc -maxdepth 3 -type f -iname '*pass*' -o -iname '*conf*' | less"
        ]
      },
      {
        title: "Repair application ownership",
        detail: "Use the application owner for its install path. Example shown for Splunk from the source checklist.",
        commands: [
          "sudo chown -R splunk:splunk /opt/splunk",
          "sudo systemctl status splunk",
          "sudo systemctl restart splunk"
        ]
      }
    ]
  },
  {
    id: "patching",
    title: "System Patching",
    summary: "Patch after access, firewall, and application impact are understood. Reboot only when planned.",
    steps: [
      {
        title: "Patch Ubuntu or Debian",
        detail: "Review apt sources first, then update and upgrade if the competition rules and scoring allow it.",
        commands: [
          "sudo find /etc/apt -maxdepth 2 -type f -ls",
          "sudo /usr/bin/apt update",
          "sudo /usr/bin/apt upgrade"
        ]
      },
      {
        title: "Patch Enterprise Linux",
        detail: "Use yum for EL7 and older, dnf for EL8 and newer. Snapshot or document before major changes.",
        commands: [
          "sudo /bin/yum update",
          "sudo /bin/dnf update",
          "sudo yum repolist"
        ]
      },
      {
        title: "Validate after patching",
        detail: "Recheck listening ports, application status, firewall rules, and scoring-critical services.",
        commands: [
          "ss -tulpan | less",
          "sudo systemctl --failed",
          "sudo journalctl -p warning -n 100"
        ]
      }
    ]
  }
];

const windowsHardeningGuide = [
  {
    id: "accounts-access",
    title: "Accounts & Access",
    summary: "Lock down local and domain privileged access before changing services or firewall rules.",
    steps: [
      {
        title: "Identify host and domain state",
        detail: "Confirm hostname, domain, IP configuration, DNS, and time source.",
        commands: [
          "hostname",
          "whoami /all",
          "ipconfig /all",
          "w32tm /query /status",
          "Get-ComputerInfo | Select-Object CsName,WindowsProductName,OsVersion"
        ]
      },
      {
        title: "Audit local users and administrators",
        detail: "Find enabled users, local administrators, stale accounts, and service accounts.",
        commands: [
          "Get-LocalUser | Select-Object Name,Enabled,LastLogon",
          "Get-LocalGroupMember Administrators",
          "net user",
          "net localgroup administrators"
        ]
      },
      {
        title: "Change privileged passwords",
        detail: "Rotate local Administrator and approved service-account passwords. Keep domain impact in mind.",
        commands: [
          "Set-LocalUser -Name \"Administrator\" -Password (Read-Host -AsSecureString \"New password\")",
          "Get-LocalUser | Where-Object Enabled -eq $true"
        ]
      },
      {
        title: "Review domain admins when available",
        detail: "On AD hosts, verify Domain Admins and Enterprise Admins membership before changing GPOs.",
        commands: [
          "net group \"Domain Admins\" /domain",
          "net group \"Enterprise Admins\" /domain",
          "Get-ADGroupMember \"Domain Admins\""
        ]
      }
    ]
  },
  {
    id: "firewall-network",
    title: "Firewall & Network",
    summary: "Enable Windows Defender Firewall and allow only ports required by the machine role.",
    steps: [
      {
        title: "Baseline listening ports",
        detail: "Map listening ports to processes before blocking traffic.",
        commands: [
          "Get-NetTCPConnection -State Listen | Sort-Object LocalPort | Select-Object LocalAddress,LocalPort,OwningProcess",
          "netstat -ano",
          "Get-Process | Select-Object Id,ProcessName,Path"
        ]
      },
      {
        title: "Review firewall profile state",
        detail: "Confirm all profiles are enabled and inbound default action is restrictive.",
        commands: [
          "Get-NetFirewallProfile | Format-Table Name,Enabled,DefaultInboundAction,DefaultOutboundAction",
          "Get-NetFirewallRule -Enabled True | Sort-Object DisplayName | Select-Object DisplayName,Direction,Action"
        ]
      },
      {
        title: "Restrict inbound access",
        detail: "Use the team subnet for administrative ports and role-specific allow rules for scoring services.",
        commands: [
          "Set-NetFirewallProfile -Profile Domain,Private,Public -Enabled True",
          "Set-NetFirewallProfile -Profile Domain,Private,Public -DefaultInboundAction Block -DefaultOutboundAction Allow",
          "New-NetFirewallRule -DisplayName \"Allow RDP from team subnet\" -Direction Inbound -Protocol TCP -LocalPort 3389 -RemoteAddress 172.20.240.0/22 -Action Allow"
        ]
      },
      {
        title: "Check DNS and routing",
        detail: "Rogue DNS or static routes can redirect services and scoring traffic.",
        commands: [
          "Get-DnsClientServerAddress",
          "Get-DnsClientCache",
          "route print"
        ]
      }
    ]
  },
  {
    id: "malware",
    title: "Malware Review",
    summary: "Use built-in Defender visibility first, then inspect persistence points.",
    steps: [
      {
        title: "Check Defender status",
        detail: "Verify real-time protection, signatures, and recent threat detections.",
        commands: [
          "Get-MpComputerStatus",
          "Update-MpSignature",
          "Start-MpScan -ScanType QuickScan",
          "Get-MpThreatDetection"
        ]
      },
      {
        title: "Inspect startup persistence",
        detail: "Review scheduled tasks, startup commands, services, and Run keys.",
        commands: [
          "Get-ScheduledTask | Where-Object State -ne Disabled | Select-Object TaskName,TaskPath,State",
          "Get-CimInstance Win32_StartupCommand | Select-Object Name,Command,Location,User",
          "Get-Service | Sort-Object Status,Name",
          "Get-ItemProperty HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
        ]
      },
      {
        title: "Review security logs",
        detail: "Look for recent privileged logons, service creation, account creation, and audit failures.",
        commands: [
          "Get-WinEvent -LogName Security -MaxEvents 100",
          "Get-WinEvent -FilterHashtable @{LogName='Security'; Id=4624,4625,4720,4732,7045} -MaxEvents 100"
        ]
      }
    ]
  },
  {
    id: "system-hardening",
    title: "System Hardening",
    summary: "Enforce local policy, audit settings, service hygiene, and file/share permissions.",
    steps: [
      {
        title: "Review local security policy",
        detail: "Export policy for review, then apply account lockout and audit settings through policy tooling.",
        commands: [
          "secedit /export /cfg C:\\Windows\\Temp\\secpol.cfg",
          "auditpol /get /category:*",
          "net accounts"
        ]
      },
      {
        title: "Check shares and permissions",
        detail: "Remove unauthorized shares and tighten access on web, FTP, DNS, and application paths.",
        commands: [
          "Get-SmbShare",
          "Get-SmbShareAccess -Name <ShareName>",
          "icacls C:\\inetpub",
          "icacls C:\\Windows\\System32\\dns"
        ]
      },
      {
        title: "Disable unused services",
        detail: "Stop only services that are unrelated to the host mission.",
        commands: [
          "Get-Service | Where-Object Status -eq Running | Sort-Object Name",
          "Stop-Service -Name <ServiceName>",
          "Set-Service -Name <ServiceName> -StartupType Disabled"
        ]
      }
    ]
  },
  {
    id: "application",
    title: "Role Hardening",
    summary: "Use the machine role to decide which services must stay reachable.",
    steps: [
      {
        title: "IIS or web server checks",
        detail: "Review bindings, app pools, web roots, and anonymous access where IIS is present.",
        commands: [
          "Import-Module WebAdministration",
          "Get-Website",
          "Get-WebBinding",
          "Get-ChildItem IIS:\\AppPools",
          "icacls C:\\inetpub\\wwwroot"
        ]
      },
      {
        title: "FTP checks",
        detail: "Validate FTP bindings, authentication methods, filesystem permissions, and allowed users.",
        commands: [
          "Get-WindowsFeature Web-Ftp-Server",
          "Get-WebConfigurationProperty -Filter /system.ftpServer/security/authentication/* -Name enabled -PSPath IIS:\\",
          "Get-Website | Where-Object Name -like '*FTP*'"
        ]
      },
      {
        title: "AD and DNS checks",
        detail: "On domain controllers, preserve AD/DNS service availability while removing unauthorized privileged access.",
        commands: [
          "Get-ADDomain",
          "Get-ADUser -Filter * -Properties Enabled,LastLogonDate | Select-Object Name,Enabled,LastLogonDate",
          "Get-DnsServerZone",
          "Get-DnsServerResourceRecord -ZoneName <ZoneName>"
        ]
      }
    ]
  },
  {
    id: "patching",
    title: "Windows Patching",
    summary: "Patch in controlled passes and validate scoring-critical services after each reboot.",
    steps: [
      {
        title: "Check update state",
        detail: "Confirm installed hotfixes and pending reboot indicators before starting patching.",
        commands: [
          "Get-HotFix | Sort-Object InstalledOn -Descending | Select-Object -First 20",
          "UsoClient StartScan",
          "Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\WindowsUpdate\\Auto Update\\RebootRequired'"
        ]
      },
      {
        title: "Validate after reboot",
        detail: "After patching, confirm required services, firewall rules, and event logs.",
        commands: [
          "Get-Service | Where-Object Status -eq Running",
          "Get-NetFirewallProfile",
          "Get-WinEvent -LogName System -MaxEvents 50"
        ]
      }
    ]
  }
];

const roleGuides = {
  ubuntuWorkstation: {
    title: "Ubuntu workstation focus",
    steps: [
      "Confirm local desktop users and remove unauthorized sudo access.",
      "Keep GUI and scoring-required services available; block unnecessary remote services.",
      "Validate SSH only if the host is expected to be managed remotely."
    ]
  },
  linuxEcommerce: {
    title: "Linux e-commerce focus",
    steps: [
      "Identify web server, database, application user, and writable web paths.",
      "Change application admin passwords and review config files for injected credentials.",
      "Keep HTTP/HTTPS reachable for scoring while restricting administrative ports."
    ]
  },
  linuxWebmail: {
    title: "Linux webmail focus",
    steps: [
      "Preserve mail/webmail ports required by scoring while blocking unrelated listeners.",
      "Audit local mail users, webmail admin accounts, and mail queue persistence.",
      "Review MTA, IMAP/POP, and webmail config files for relay or rogue admin changes."
    ]
  },
  splunk: {
    title: "Splunk focus",
    steps: [
      "Change Splunk admin credentials and audit local OS accounts that can control Splunk.",
      "Keep scoring-required Splunk web and management ports available.",
      "Verify /opt/splunk ownership and restart Splunk only after config validation."
    ],
    commands: [
      "sudo chown -R splunk:splunk /opt/splunk",
      "sudo systemctl status splunk",
      "sudo ss -tulpan | egrep '8000|8089|9997|514'"
    ]
  },
  windowsFtp: {
    title: "Windows FTP focus",
    steps: [
      "Validate FTP authentication, authorized users, and root folder permissions.",
      "Allow FTP ports needed by scoring, then restrict RDP and SMB to trusted ranges.",
      "Remove anonymous or unauthorized write access unless the mission requires it."
    ]
  },
  windowsAdDns: {
    title: "AD / DNS focus",
    steps: [
      "Audit Domain Admins, Enterprise Admins, DNS admins, and recent account changes.",
      "Preserve AD, Kerberos, LDAP, SMB, RPC, and DNS reachability required by domain clients.",
      "Review DNS zones for rogue records and protect Group Policy from unauthorized edits."
    ]
  },
  windowsWeb: {
    title: "Windows web focus",
    steps: [
      "Review IIS bindings, application pools, web root ACLs, and anonymous authentication.",
      "Keep HTTP/HTTPS available for scoring and restrict administrative ports.",
      "Search web roots for web shells, unexpected scripts, and writable upload paths."
    ]
  }
};

const linuxResources = {
  vcpus: 2,
  ramMb: 4096,
  diskGb: 40,
  networks: 2,
  servers: 1
};

const windowsResources = {
  vcpus: 2,
  ramMb: 6144,
  diskGb: 60,
  networks: 2,
  servers: 1
};

export const labs = [
  {
    id: "cirros-smoke-test",
    slug: "cirros-smoke-test",
    name: "Cirros Smoke Test",
    description: "Small OpenStack deploy test",
    difficulty: "OpenStack test",
    category: "test",
    platform: "Test",
    defaultTtlMinutes: 60,
    enabled: true,
    resources: {
      vcpus: 1,
      ramMb: 64,
      diskGb: 1,
      networks: 1,
      servers: 1
    },
    network: {
      lan: "private",
      public: "noVNC"
    },
    credentials: [
      { username: "cirros", password: "gocubsgo" }
    ],
    accessMethods: ["SSH", "Console"],
    hardeningSteps: [
      "Confirm the VM builds, receives DHCP, and gets a floating IP.",
      "Validate SSH or console login with the Cirros smoke-test credentials.",
      "Destroy the stack after the provider path is verified."
    ],
    hardeningGuide: [],
    roleGuide: null,
    heatTemplatePath: "heat-templates/single-cirros.yaml"
  },
  {
    id: "ccdc-wkst-ubuntu-24",
    slug: "ccdc-wkst-ubuntu-24",
    name: "Wkst Ubuntu 24",
    description: "Desktop testing host",
    difficulty: "CCDC Linux",
    category: "linux",
    platform: "Linux",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: linuxResources,
    network: {
      lan: "DHCP",
      public: "dynamic"
    },
    credentials: [
      { username: "sysadmin", password: "changeme" }
    ],
    accessMethods,
    hardeningSteps: linuxHardeningSteps,
    hardeningGuide: linuxHardeningGuide,
    roleGuide: roleGuides.ubuntuWorkstation,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-ecom-ubuntu-24",
    slug: "ccdc-ecom-ubuntu-24",
    name: "Ecom Ubuntu 24",
    description: "Storefront",
    difficulty: "CCDC Linux",
    category: "linux",
    platform: "Linux",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: linuxResources,
    network: {
      lan: "172.20.242.104",
      public: "172.25.39.11"
    },
    credentials: [
      { username: "sysadmin", password: "changeme" }
    ],
    accessMethods,
    hardeningSteps: linuxHardeningSteps,
    hardeningGuide: linuxHardeningGuide,
    roleGuide: roleGuides.linuxEcommerce,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-webmail-fedora-42",
    slug: "ccdc-webmail-fedora-42",
    name: "Webmail Fedora 42",
    description: "Messaging endpoint",
    difficulty: "CCDC Linux",
    category: "linux",
    platform: "Linux",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: linuxResources,
    network: {
      lan: "172.20.242.101",
      public: "172.25.39.39"
    },
    credentials: [
      { username: "sysadmin", password: "changeme" }
    ],
    accessMethods,
    hardeningSteps: linuxHardeningSteps,
    hardeningGuide: linuxHardeningGuide,
    roleGuide: roleGuides.linuxWebmail,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-splunk",
    slug: "ccdc-splunk",
    name: "Splunk",
    description: "Security analytics",
    difficulty: "CCDC Linux",
    category: "linux",
    platform: "Linux",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: {
      ...linuxResources,
      vcpus: 4,
      ramMb: 8192,
      diskGb: 80
    },
    network: {
      lan: "172.20.242.20",
      public: "172.25.39.9"
    },
    credentials: [
      { username: "root", password: "changemenow" },
      { username: "sysadmin", password: "changemenow" },
      { username: "admin", password: "changeme" }
    ],
    accessMethods,
    hardeningSteps: linuxHardeningSteps,
    hardeningGuide: linuxHardeningGuide,
    roleGuide: roleGuides.splunk,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-ftp-server-2022",
    slug: "ccdc-ftp-server-2022",
    name: "FTP Server 2022",
    description: "File transfer staging",
    difficulty: "CCDC Windows",
    category: "windows",
    platform: "Windows",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: windowsResources,
    network: {
      lan: "172.20.240.104",
      public: "172.25.39.162"
    },
    credentials: [
      { username: "administrator", password: "!Password123" }
    ],
    accessMethods,
    hardeningSteps: windowsHardeningSteps,
    hardeningGuide: windowsHardeningGuide,
    roleGuide: roleGuides.windowsFtp,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-ad-dns-server-2019",
    slug: "ccdc-ad-dns-server-2019",
    name: "AD / DNS Server 2019",
    description: "Identity & name services",
    difficulty: "CCDC Windows",
    category: "windows",
    platform: "Windows",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: {
      ...windowsResources,
      vcpus: 4,
      ramMb: 8192
    },
    network: {
      lan: "172.20.240.102",
      public: "172.25.39.155"
    },
    credentials: [
      { username: "administrator", password: "!Password123" }
    ],
    accessMethods,
    hardeningSteps: windowsHardeningSteps,
    hardeningGuide: windowsHardeningGuide,
    roleGuide: roleGuides.windowsAdDns,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  },
  {
    id: "ccdc-web-server-2019",
    slug: "ccdc-web-server-2019",
    name: "Web Server 2019",
    description: "Public web tier",
    difficulty: "CCDC Windows",
    category: "windows",
    platform: "Windows",
    defaultTtlMinutes: 240,
    enabled: true,
    resources: windowsResources,
    network: {
      lan: "172.20.240.101",
      public: "172.25.39.140"
    },
    credentials: [
      { username: "administrator", password: "!Password123" }
    ],
    accessMethods,
    hardeningSteps: windowsHardeningSteps,
    hardeningGuide: windowsHardeningGuide,
    roleGuide: roleGuides.windowsWeb,
    heatTemplatePath: "heat-templates/mini-ccdc.yaml"
  }
];

export function findEnabledLab(labId) {
  return labs.find((lab) => lab.enabled && lab.id === labId);
}
