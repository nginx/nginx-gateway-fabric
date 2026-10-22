#!/bin/bash

set -euxo pipefail

nginx_pid=""
agent_pid=""

stop_process() {
    local signal="$1"
    local pid="$2"

    if [[ -n $pid ]] && kill -0 "$pid" 2>/dev/null; then
        kill "-$signal" "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    fi
}

handle_term() {
    echo "received TERM signal"
    echo "stopping nginx-agent ..."
    stop_process TERM "$agent_pid"
    echo "stopping nginx ..."
    stop_process TERM "$nginx_pid"
    exit 0
}

handle_quit() {
    echo "received QUIT signal"
    echo "stopping nginx-agent ..."
    stop_process QUIT "$agent_pid"
    echo "stopping nginx ..."
    stop_process QUIT "$nginx_pid"
    exit 0
}

trap 'handle_term' TERM
trap 'handle_quit' QUIT

rm -rf /var/run/nginx/*.sock

# Bootstrap the necessary app protect files
if [ "${USE_NAP_WAF:-false}" = "true" ]; then
    touch /opt/app_protect/bd_config/policy_path.map
    touch /opt/app_protect/config/config_set.json
fi

# Launch nginx
echo "starting nginx ..."

# if we want to use the nginx-debug binary, we will call this script with an argument "debug"
if [ "${1:-false}" = "debug" ]; then
    /usr/sbin/nginx-debug -g "daemon off;" &
else
    /usr/sbin/nginx -g "daemon off;" &
fi

nginx_pid=$!

SECONDS=0
while [[ ! -f /var/run/nginx.pid ]] && [[ ! -f /var/run/nginx/nginx.pid ]]; do
    if ((SECONDS > 30)); then
        echo "couldn't find nginx master process"
        exit 1
    fi
    sleep 1
done

# start nginx-agent, pass args
echo "starting nginx-agent ..."
GOMEMLIMIT=150MiB GOGC=75 nginx-agent &

agent_pid=$!

if ! kill -0 "$agent_pid" 2>/dev/null; then
    echo "couldn't start the agent, please check the log file"
    exit 1
fi

wait_term() {
    # Exit if either process stops so Kubernetes can restart the container when
    # the nginx master dies while orphaned workers remain alive.
    wait -n "$agent_pid" "$nginx_pid"

    if kill -0 "$nginx_pid" 2>/dev/null; then
        trap - TERM
        stop_process QUIT "$nginx_pid"
        echo "waiting for nginx to stop..."
        return 0
    fi

    echo "nginx master process has stopped, stopping nginx-agent..."
    stop_process TERM "$agent_pid"
    return 1
}

if wait_term; then
    echo "nginx-agent process has stopped, exiting."
else
    echo "nginx master process has stopped, exiting."
    exit 1
fi
