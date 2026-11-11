#!/bin/sh
set -eu

newsletter validate-config
newsletter generate-crontab > /tmp/newsletter-crontab
supercronic -test /tmp/newsletter-crontab
exec supercronic /tmp/newsletter-crontab
