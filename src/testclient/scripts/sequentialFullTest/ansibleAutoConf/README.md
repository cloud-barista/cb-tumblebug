# Files for Ansible usecase

This directory includes files related with Ansible

***

## [Playbooks]

### helloworld.yml
- default playbook example
- returns `Hello world!`
- ex: `ansible-playbook ./ansibleAutoConf/helloworld.yml -i ./ansibleAutoConf/mcis-shson05-host`

### deploy-nginx-web-server.yml
- default nginx deployment example
- deploy nginx server to all VMs in MCIS
- prerequisite `ansible-galaxy collection install nginxinc.nginx_core`
- ex: `ansible-playbook ./ansibleAutoConf/deploy-nginx-web-server.yml -i ./ansibleAutoConf/mcis-shson05-host`

### add-key.yml
- put a public key to all VMs in MCIS
