#!/bin/bash

set -euxo pipefail

handle_term() {
    echo "received TERM signal"
    echo "stopping nginx-agent ..."
    kill -TERM "${agent_pid}" 2>/dev/null
    wait -n ${agent_pid}
    echo "stopping nginx ..."
    kill -TERM "${nginx_pid}" 2>/dev/null
    wait -n ${nginx_pid}
}

handle_quit() {
    echo "received QUIT signal"
    echo "stopping nginx-agent ..."
    kill -QUIT "${agent_pid}" 2>/dev/null
    wait -n ${agent_pid}
    echo "stopping nginx ..."
    kill -QUIT "${nginx_pid}" 2>/dev/null
    wait -n ${nginx_pid}
}

trap 'handle_term' TERM
trap 'handle_quit' QUIT

rm -rf /var/run/nginx/*.sock

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

while ! ps -ef | grep "nginx: master process" | grep -v grep; do
    if ((SECONDS > 5)); then
        echo "couldn't find nginx master process"
        exit 1
    fi
done

# start nginx-agent, pass args
echo "starting nginx-agent ..."
nginx-agent &

agent_pid=$!

if [ $? != 0 ]; then
    echo "couldn't start the agent, please check the log file"
    exit 1
fi

wait_term() {
    wait ${agent_pid}
    trap - TERM
    kill -QUIT "${nginx_pid}" 2>/dev/null
    echo "waiting for nginx to stop..."
    wait ${nginx_pid}
}

wait_term

echo "nginx-agent process has stopped, exiting."
