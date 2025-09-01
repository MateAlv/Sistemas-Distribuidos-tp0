#!/bin/bash

SERVER_PORT=12345
SERVER_IP=server
# SERVER_LISTEN_BACKLOG=5
# LOGGING_LEVEL=INFO
NETWORK_NAME="tp0_testing_net"
MESSAGE="Hola Mate"

RESPONSE=$(echo "${MESSAGE}" | docker run --rm --network=${NETWORK_NAME} appropriate/nc ${SERVER_IP} ${SERVER_PORT})

if [[ "${RESPONSE}" == "${MESSAGE}" ]]; then
  echo "action: test_echo_server | result: success"
else
  echo "action: test_echo_server | result: fail"
fi
