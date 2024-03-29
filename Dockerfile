# syntax=docker/dockerfile:1
FROM ubuntu:latest

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get -qq update -y && apt-get -qq upgrade -y

RUN apt-get -qq install binutils software-properties-common libcap2-bin apt-utils wget curl htop iputils-ping dnsutils -y
RUN add-apt-repository multiverse
RUN dpkg --add-architecture i386

RUN apt-get -qq install lib32gcc-s1 -y 
RUN apt-get -qq update -y

RUN useradd -m -u 9999 -s /bin/bash ssm 

RUN mkdir /opt/SSMAgent
VOLUME /opt/SSMAgent
RUN ls -l
COPY release/linux/* /opt/SSMAgent/
RUN chown -R ssm:ssm /opt/SSMAgent

RUN mkdir -p /home/ssm/SSM/Agents && mkdir -p /home/ssm/.config/Epic/FactoryGame && mkdir -p /SSM/data
RUN chown -R ssm:ssm /home/ssm
RUN chown -R ssm:ssm /SSM/data

COPY entry.sh /entry.sh
RUN chmod 755 /entry.sh

RUN ls -l /

ENTRYPOINT [ "/entry.sh" ]
