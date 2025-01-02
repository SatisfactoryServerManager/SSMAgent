#!/bin/bash

export DEBIAN_FRONTEND=noninteractive

apt-get -qq update -y && apt-get -qq upgrade -y

apt-get -qq install binutils software-properties-common libcap2-bin apt-utils wget curl htop iputils-ping dnsutils -y

add-apt-repository multiverse
dpkg --add-architecture i386

apt-get -qq install lib32gcc-s1 -y
apt-get -qq update -y

apt-get -qq install -y wget apt-transport-https software-properties-common
source /etc/os-release
wget -q https://packages.microsoft.com/config/ubuntu/$VERSION_ID/packages-microsoft-prod.deb
dpkg -i packages-microsoft-prod.deb
rm packages-microsoft-prod.deb
apt-get -qq update -y
apt-get -qq install -y powershell

useradd -m -u 9999 -s /bin/bash ssm

mkdir -p /opt/SSMAgent && mkdir -p /home/ssm/SSM/Agents && mkdir -p /home/ssm/.config/Epic/FactoryGame && mkdir -p /SSM/data

chown -R ssm:ssm /opt/SSMAgent
chown -R ssm:ssm /home/ssm
chown -R ssm:ssm /SSM/data
