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
if [ -n "${SEATUNNEL_CAPABILITY_PROXY_HOME:-}" ] && [ -d "${SEATUNNEL_CAPABILITY_PROXY_HOME}" ]; then
  PROXY_HOME="${SEATUNNEL_CAPABILITY_PROXY_HOME}"
fi
if [ -z "${SEATUNNEL_HOME:-}" ] && [ -f "${PROXY_HOME}/starter/seatunnel-starter.jar" ]; then
  SEATUNNEL_HOME="${PROXY_HOME}"
fi

APP_JAR=${SEATUNNEL_HOME:-}/starter/seatunnel-starter.jar
DEFAULT_PROXY_VERSION="${CAPABILITY_PROXY_DEFAULT_VERSION:-2.3.13}"
APP_MAIN="org.apache.seatunnel.tools.proxy.CapabilityProxyApplication"

proxy_version_candidates() {
  local requested_version="${SEATUNNEL_CAPABILITY_PROXY_VERSION:-${SEATUNNEL_VERSION:-}}"
  if [ -n "${requested_version}" ]; then
    printf '%s\n' "${requested_version}"
  fi
  if [ "${requested_version}" != "${DEFAULT_PROXY_VERSION}" ]; then
    printf '%s\n' "${DEFAULT_PROXY_VERSION}"
  fi
}

find_proxy_jar() {
  local version candidate
  while IFS= read -r version; do
    [ -z "${version}" ] && continue

    candidate="${PROXY_HOME}/lib/seatunnel-capability-proxy-${version}.jar"
    if [ -f "${candidate}" ]; then
      echo "${candidate}"
      return 0
    fi

    candidate=$(find "${PROXY_HOME}/tools/seatunnel-capability-proxy/target" -maxdepth 1 -type f -name "seatunnel-capability-proxy-${version}*.jar" 2>/dev/null | grep -v '\-bin\.jar$' | sort | head -n 1 || true)
    if [ -n "${candidate}" ]; then
      echo "${candidate}"
      return 0
    fi
  done < <(proxy_version_candidates)

  if [ -f "${PROXY_HOME}/lib/seatunnel-capability-proxy.jar" ]; then
    echo "${PROXY_HOME}/lib/seatunnel-capability-proxy.jar"
    return 0
  fi

  find "${PROXY_HOME}/tools/seatunnel-capability-proxy/target" -maxdepth 1 -type f -name 'seatunnel-capability-proxy-*.jar' 2>/dev/null | grep -v '\-bin\.jar$' | sort | head -n 1 || true
}

DEFAULT_PROXY_JAR="$(find_proxy_jar)"
PROXY_JAR=${SEATUNNEL_PROXY_JAR:-${DEFAULT_PROXY_JAR}}

if [ ! -f "${APP_JAR}" ]; then
  echo "seatunnel-starter.jar not found under ${SEATUNNEL_HOME:-<unset>}/starter; please set SEATUNNEL_HOME" >&2
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
