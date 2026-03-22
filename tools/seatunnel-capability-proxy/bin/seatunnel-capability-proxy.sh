#!/bin/bash
#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -eu

PRG="$0"
while [ -h "$PRG" ]; do
  ls_output=$(ls -ld "$PRG")
  link=$(expr "$ls_output" : '.*-> \(.*\)$')
  if expr "$link" : '/.*' > /dev/null; then
    PRG="$link"
  else
    PRG=$(dirname "$PRG")/"$link"
  fi
done

PRG_DIR=$(dirname "$PRG")
PROXY_HOME=$(cd "$PRG_DIR/.." >/dev/null; pwd)
if [ -z "${SEATUNNEL_HOME:-}" ]; then
  SEATUNNEL_HOME=$(cd "$PROXY_HOME/../.." >/dev/null; pwd)
fi

APP_JAR=${SEATUNNEL_HOME}/starter/seatunnel-starter.jar
DEFAULT_PROXY_JAR=$(ls "${PROXY_HOME}"/target/seatunnel-capability-proxy-*.jar 2>/dev/null | grep -v '\-bin\.jar$' | head -n 1 || true)
PROXY_JAR=${SEATUNNEL_PROXY_JAR:-${DEFAULT_PROXY_JAR}}
APP_MAIN="org.apache.seatunnel.tools.proxy.CapabilityProxyApplication"

if [ ! -f "${APP_JAR}" ]; then
  echo "seatunnel-starter.jar not found under ${SEATUNNEL_HOME}/starter" >&2
  exit 1
fi

if [ ! -f "${PROXY_JAR}" ]; then
  echo "proxy jar not found: ${PROXY_JAR}" >&2
  exit 1
fi

if [ -f "${SEATUNNEL_HOME}/config/seatunnel-env.sh" ]; then
  # shellcheck disable=SC1091
  . "${SEATUNNEL_HOME}/config/seatunnel-env.sh"
fi

JAVA_OPTS=${JAVA_OPTS:-}
APP_ARGS=()
for arg in "$@"; do
  if [[ "${arg}" == -D* ]]; then
    JAVA_OPTS="${JAVA_OPTS} ${arg}"
  else
    APP_ARGS+=("${arg}")
  fi
done
JAVA_OPTS="${JAVA_OPTS} -Dseatunnel.capability.proxy.seatunnel.home=${SEATUNNEL_HOME}"

CLASS_PATH=${SEATUNNEL_HOME}/lib/*:${APP_JAR}:${PROXY_JAR}

append_plugin_jars() {
  local plugin_dir="$1"
  local jar_list_file
  if [ ! -d "${plugin_dir}" ]; then
    return
  fi
  jar_list_file=$(mktemp)
  find "${plugin_dir}" -type f -name '*.jar' | sort > "${jar_list_file}"
  while IFS= read -r jar_path; do
    CLASS_PATH=${CLASS_PATH}:${jar_path}
  done < "${jar_list_file}"
  rm -f "${jar_list_file}"
}

if [ -d "${SEATUNNEL_HOME}/connectors" ]; then
  CLASS_PATH=${CLASS_PATH}:${SEATUNNEL_HOME}/connectors/*
fi
if [ -d "${SEATUNNEL_HOME}/plugins" ]; then
  for plugin_dir in "${SEATUNNEL_HOME}"/plugins/*; do
    if [ -d "${plugin_dir}" ]; then
      append_plugin_jars "${plugin_dir}"
    fi
  done
fi
if [ -n "${EXTRA_PROXY_CLASSPATH:-}" ]; then
  CLASS_PATH=${CLASS_PATH}:${EXTRA_PROXY_CLASSPATH}
fi

if [ ${#APP_ARGS[@]} -eq 0 ]; then
  exec java ${JAVA_OPTS} -cp "${CLASS_PATH}" ${APP_MAIN}
fi
exec java ${JAVA_OPTS} -cp "${CLASS_PATH}" ${APP_MAIN} "${APP_ARGS[@]}"
