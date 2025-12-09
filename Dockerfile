# syntax=docker/dockerfile:1
FROM ubuntu:latest

COPY scripts/setup-docker.sh /setup-docker.sh

RUN chmod +x /setup-docker.sh
RUN /setup-docker.sh

VOLUME /opt/SSMAgent
COPY release/linux/* /opt/SSMAgent/

RUN ls -l /opt/SSMAgent/

COPY entry.sh /entry.sh
RUN chmod 755 /entry.sh

RUN ls -l /

ENTRYPOINT [ "/entry.sh" ]
