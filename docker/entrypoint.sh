#!/bin/sh
set -eu

case "${1:-scheduler}" in
serve)
	shift
	exec newsletter serve "$@"
	;;
scheduler)
	;;
*)
	exec newsletter "$@"
	;;
esac

newsletter validate-config
newsletter generate-crontab > /tmp/newsletter-crontab
supercronic -test /tmp/newsletter-crontab
exec supercronic /tmp/newsletter-crontab
