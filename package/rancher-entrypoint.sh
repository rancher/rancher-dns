#!/bin/bash

/usr/bin/update-rancher-ssl

METADATA_SERVER=${METADATA_SERVER:-localhost}
RANCHER_METADATA_ANSWER=${RANCHER_METADATA_ANSWER:-169.254.169.250}
NEVER_RECURSE_TO=${NEVER_RECURSE_TO:-169.254.169.250}
AGENT_IP=

load_agent_ip() {
  # loop until metadata is available
  while [ "$AGENT_IP" == "" ] || [ "$AGENT_IP" == "Not found" ]; do
    AGENT_IP=$(curl -s $METADATA_SERVER/2016-07-29/self/host/agent_ip)
    sleep 1
  done
}

if [ "$RANCHER_METADATA_ANSWER" == "agent_ip" ]; then
  load_agent_ip
  RANCHER_METADATA_ANSWER=$AGENT_IP
fi

if [ "$NEVER_RECURSE_TO" == "agent_ip" ]; then
  load_agent_ip
  NEVER_RECURSE_TO=$AGENT_IP
fi

exec $@ \
  -rancher-metadata-answer="$RANCHER_METADATA_ANSWER" \
  -never-recurse-to="$NEVER_RECURSE_TO"
