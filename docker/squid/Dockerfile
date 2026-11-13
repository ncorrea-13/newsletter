# Copyright (c) 2026 Nicolás Correa
# SPDX-License-Identifier: MIT

FROM debian:trixie-slim

# --no-install-recommends evita exim4 y otros paquetes que fallan en el postinst.
RUN apt-get update \
 && apt-get install -y --no-install-recommends squid \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /var/spool/squid /var/log/squid \
 && chown proxy:proxy /var/spool/squid /var/log/squid

COPY squid.conf /etc/squid/squid.conf

EXPOSE 3128

ENTRYPOINT ["squid", "-N", "-f", "/etc/squid/squid.conf"]
