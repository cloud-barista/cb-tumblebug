#!/bin/bash

# Install using the convenience script
# Docker provides a convenience script at get.docker.com to install Docker into development
# environments quickly and non-interactively. The convenience script is not recommended for
# production environments, but can be used as an example to create a provisioning script that
# is tailored to your needs. Also refer to the install using the repository steps to
# learn about installation steps to install using the package repository.
# The source code for the script is open source, and can be found in the docker-install repository on GitHub.

# OS requirement (one of the followings)
# Ubuntu Jammy 22.04 (LTS)
# Ubuntu Impish 21.10
# Ubuntu Focal 20.04 (LTS)
# Ubuntu Bionic 18.04 (LTS)
# Debian Bullseye 11 (stable)
# Debian Buster 10 (oldstable)
# Raspbian Bullseye 11 (stable)
# Raspbian Buster 10 (oldstable)
# Fedora 34
# Fedora 35
# Fedora 36
# CentOS 7, CentOS 8 (stream), or CentOS 9 (stream)
# RHEL 7, RHEL 8 or RHEL 9 on s390x (IBM Z)

curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo docker -v
